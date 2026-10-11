package interp

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"eyelang/ir"
)

// ---------------------------------------------------------------- faults a run can end with

// runFault stops the run, the test or the program with one of the prelude's
// Fault variants (DeadlinePassed, OverBudget, OutOfMemory, …): an outcome the
// program declared, which ends a job's run as Failed(kind) and doesn't fail
// the test it ran in.
func runFault(at ir.Pos, kind, format string, args ...any) {
	panic(&Fault{Pos: at, Msg: fmt.Sprintf(format, args...), Kind: kind})
}

// ---------------------------------------------------------------- budgets

// spend is what model calls used: tokens, and their price in US dollars.
type spend struct {
	Tokens int64
	USD    float64
}

// budget is a @budget being kept: the limits of one job run, or of one call
// of a function, and what the model calls under it spent so far. Tasks share
// it, so it has a lock.
type budget struct {
	of    string // "job digest", "summarise": who declared it, for messages
	limit *ir.Budget
	mu    sync.Mutex
	spent spend
}

// underBudget runs body, a call of fn, with fn's @budget kept on top of the
// budgets of its callers. A function's budget is for each call of it; a
// job's, for its run, which counts what its replayed steps spent again, so
// the budget holds across restarts.
func (in *Interp) underBudget(fn *ir.Func, body func() Value) Value {
	of := fn.Name
	if job, ok := fn.Module.Jobs[fn.Name]; ok && job.Func == fn {
		of = "job " + fn.Name
	}
	prev := in.budgets
	in.budgets = append(prev[:len(prev):len(prev)], &budget{of: of, limit: fn.Budget})
	defer func() { in.budgets = prev }()
	return body()
}

// charge adds what a model call spent to every budget being kept.
func (in *Interp) charge(s spend) {
	if s == (spend{}) {
		return
	}
	for _, b := range in.budgets {
		b.mu.Lock()
		b.spent.Tokens += s.Tokens
		b.spent.USD += s.USD
		b.mu.Unlock()
	}
	if in.stepSpend != nil {
		in.stepSpend.Tokens += s.Tokens
		in.stepSpend.USD += s.USD
	}
}

// minReply is the fewest reply tokens worth sending a call for: with less
// left, the reply would be cut off before it said anything.
const minReply = 64

// defaultMaxReply is how long a reply may be when no budget limits it.
const defaultMaxReply = 16000

// allowance is how many tokens a model's reply may take, so that no budget
// being kept goes over, given that the prompt takes about prompt tokens. It
// ends the run as Failed(OverBudget) when a budget can't fit the prompt and
// a reply of minReply tokens, and the call isn't sent. capped says a budget,
// not the default, set the limit.
func (in *Interp) allowance(model string, prompt int64, at ir.Pos) (limit int64, capped bool) {
	limit = defaultMaxReply
	for _, b := range in.budgets {
		b.mu.Lock()
		spent := b.spent
		b.mu.Unlock()
		if in.rt.aiReplies != nil {
			// Recorded replies cost what their recordings say, and nothing
			// is sent, so a budget only needs room left.
			if b.limit.Tokens >= 0 && spent.Tokens >= b.limit.Tokens || b.limit.USD >= 0 && spent.USD >= b.limit.USD {
				runFault(at, "OverBudget", "%s has spent its budget, and the next call to %s isn't sent", b.of, model)
			}
			continue
		}
		if b.limit.Tokens >= 0 {
			if left := b.limit.Tokens - spent.Tokens - prompt; left < limit {
				limit, capped = left, true
			}
			if limit < minReply {
				runFault(at, "OverBudget", "%s has spent %d of its %d tokens, and the next call to %s needs about %d more",
					b.of, spent.Tokens, b.limit.Tokens, model, prompt+minReply)
			}
		}
		if b.limit.USD >= 0 {
			p, ok := priceOf(model)
			if !ok {
				in.faultf(at, "@budget(usd: %g) can't be kept, since the price of %s isn't known: give it in AI_PRICES, such as AI_PRICES=\"%s=3/15\" for $3 per million tokens in and $15 out, or limit the budget by tokens", b.limit.USD, model, model)
			}
			left := b.limit.USD - spent.USD - p.cost(prompt, 0)
			if n := int64(math.Floor(left / p.Out * 1e6)); n < limit {
				limit, capped = n, true
			}
			if limit < minReply {
				runFault(at, "OverBudget", "%s has spent $%.4f of its $%.2f, and the next call to %s would cost about $%.4f more",
					b.of, spent.USD, b.limit.USD, model, p.cost(prompt, minReply))
			}
		}
	}
	return limit, capped
}

// ---------------------------------------------------------------- prices

// price is what a model costs, in US dollars per million tokens.
type price struct{ In, Out float64 }

func (p price) cost(in, out int64) float64 {
	return (float64(in)*p.In + float64(out)*p.Out) / 1e6
}

// prices are the models whose price is known, by name. A deployment gives
// others, or newer prices, in AI_PRICES: "model=in/out,…".
var prices = map[string]price{
	"claude-fable-5-1":  {10, 50},
	"claude-fable-5":    {10, 50},
	"claude-opus-5-5":   {4, 20},
	"claude-opus-5":     {5, 25},
	"claude-opus-4-8":   {5, 25},
	"claude-opus-4-7":   {5, 25},
	"claude-opus-4-6":   {5, 25},
	"claude-sonnet-5-5": {2, 10},
	"claude-sonnet-5":   {2, 10},
	"claude-sonnet-4-6": {3, 15},
	"claude-haiku-5-5":  {0.10, 0.50},
	"claude-haiku-4-5":  {1, 5},
}

// priceOf finds a model's price: in AI_PRICES, then in the table. A name a
// gateway gives with its provider, "anthropic/claude-opus-5-5", is found by
// the part after the slash.
func priceOf(model string) (price, bool) {
	names := []string{model}
	if i := strings.LastIndexByte(model, '/'); i >= 0 {
		names = append(names, model[i+1:])
	}
	for _, entry := range strings.Split(os.Getenv("AI_PRICES"), ",") {
		name, rates, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok {
			continue
		}
		in, out, ok := strings.Cut(rates, "/")
		pin, err1 := strconv.ParseFloat(strings.TrimSpace(in), 64)
		pout, err2 := strconv.ParseFloat(strings.TrimSpace(out), 64)
		if !ok || err1 != nil || err2 != nil {
			continue
		}
		for _, n := range names {
			if strings.TrimSpace(name) == n {
				return price{pin, pout}, true
			}
		}
	}
	for _, n := range names {
		if p, ok := prices[n]; ok {
			return p, true
		}
	}
	return price{}, false
}
