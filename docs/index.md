---
layout: home
title: Eyelang
titleTemplate: false

hero:
  name: Eyelang
  text: For software that keeps state and waits
  tagline: Services, automations, schedulers and agents that run for days, hold data, call systems that fail, and spend most of their time waiting. As simple to write as a script, and correct by construction.
  actions:
    - theme: brand
      text: Quickstart
      link: /quickstart
    - theme: alt
      text: Why Eyelang
      link: /why
    - theme: alt
      text: Tutorial
      link: /tutorial/

features:
  - title: Values, not shared objects
    details: Data is immutable, mutation is local and marked with mut, and nothing is aliased. Parallel code has no data races, and memory needs no garbage collector.
    link: /reference/language/mutation
  - title: State has an owner
    details: Data that outlives a call lives in an entity, and only that entity's methods change it, one transaction at a time.
    link: /reference/language/entities
  - title: Waiting is free and durable
    details: A job pauses at a ! step for minutes or months. While it waits it is a saved record, not a thread, and it survives restarts.
    link: /reference/language/jobs-and-waiting
  - title: Typed boundaries, explicit failure
    details: Contracts type the calls to other services, calls are keyed so they never happen twice, and failures are Result values that matches must handle.
    link: /reference/language/contracts-talking-to-other-services
  - title: Costs and effects are visible
    details: uses in a signature lists its effects, ! marks every durable step, mut every change and dyn every run-time lookup. A reviewer sees them in the diff.
    link: /reference/language/effects-and-signatures
  - title: Tests that can't flake
    details: Each test gets a fresh store and a clock that moves only when the test says so. Calls to other services are replayed from recordings.
    link: /reference/language/time-secrets-tests
---

## A job that waits

An idle game world goes to sleep after 20 minutes, unless someone joins first. The job is ordinary
code: `sleep!` pauses it for 20 minutes without holding memory, and the checkpoint call is keyed, so
a restart can't take it twice.

```eye
@on(enters(idleWorlds))
job sleepWhenIdle(w: Ref(World)) -> Result((), SleepError) {
  sleep!(20 min)
  let world = w.read!()
  if world.players != {} {
    return Ok(())
  }
  match zeropod.checkpoint!(w.id, key: "sleep-{w.id}-{world.version}").reconcile!() {
    Done(_) => {
      w.markAsleep!()
      Ok(())
    }
    Refused(reason) => Err(SleepRefused(reason))
  }
}
```

The [tour](/reference/language/a-tour) shows the whole program: the entity, the view that finds idle
worlds, and a test that runs those 20 minutes in no time.

::: info Eyelang is being built
The language changes whenever real programs show a better way, until v1.0. A reference interpreter
runs programs today; native compilation comes later. See the [design](/design/) for the goals and the
open questions.
:::
