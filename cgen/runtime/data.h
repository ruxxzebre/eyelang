/* Values at the program's boundary: JSON (json.c), saved values (save.c)
 * and calls of host functions the Go runner answers (host.c). */
#ifndef EYE_DATA_H
#define EYE_DATA_H

#include "eye.h"

/* toJson of the value of type t at p (SPECS.md §17, "Values as JSON") */
ey_v ey_to_json(const char *at, const ey_type *t, const void *p);

/* ---------------------------------------------------------------- saved values (save.c) */

/* Saved bytes being read: where the next byte is, where they end, and
 * what was wrong with them once a read failed. */
typedef struct ey_rd {
	const uint8_t *p, *end;
	const char *err;
} ey_rd;

/* The encoder and the decoder the back end writes for each saved type
 * (cgen/save.go). An encoder appends the value at v; a decoder reads one
 * into out, which starts zeroed, and gives false, with r->err set, for
 * bytes that aren't a value of its type. */
typedef void (*ey_encoder)(ey_buf *b, const void *v);
typedef bool (*ey_decoder)(ey_rd *r, void *out);

/* A type the runner hands to compiled code or takes from it (7.6), by its
 * name (cgen.SavedName: lir.TypeKey's, such as "List(main.Point)"): its
 * descriptor, which says how large a value is and how to drop one, and its
 * encoder and decoder. Every program has the table, empty or not. */
typedef struct ey_saved {
	const char *name;
	const ey_type *ty;
	ey_encoder encode;
	ey_decoder decode;
} ey_saved;
extern const ey_saved ey_saved_types[];
extern const int ey_nsaved;
const ey_saved *ey_saved_find(const char *name);
/* ey_save appends the bytes of the value at v; ey_load reads n bytes of
 * data into out, which owns what it reads, and gives NULL, or what is
 * wrong with the bytes (and out is left zero). */
void ey_save(const ey_saved *s, const void *v, ey_buf *out);
const char *ey_load(const ey_saved *s, const char *data, int64_t n, void *out);

/* What the encoders write with. */
void eys_uvarint(ey_buf *b, uint64_t n);
void eys_varint(ey_buf *b, int64_t n);
void eys_text(ey_buf *b, const char *s, int64_t n); /* its length, then its bytes */
void eys_tagged(ey_buf *b, char tag, int64_t n);    /* a tag, then a varint */
void eys_int(ey_buf *b, ey_v n);
void eys_fixed(ey_buf *b, const ey_type *t, const void *p);
void eys_float(ey_buf *b, double d);
void eys_string(ey_buf *b, ey_v s);
void eys_bytes(ey_buf *b, ey_v s);
void eys_date(ey_buf *b, int64_t days);
void eys_time(ey_buf *b, int64_t secs);
void eys_instant(ey_buf *b, ey_v t);
void eys_list(ey_buf *b, ey_v l, ey_encoder e);
void eys_set(ey_buf *b, ey_v s, ey_encoder e);
void eys_map(ey_buf *b, ey_v m, ey_encoder ke, ey_encoder ve);
void eys_range(ey_buf *b, ey_v r, const ey_type *elem, ey_encoder e);

/* What the decoders read with. Each gives false, with r->err set, for
 * bytes that aren't what it reads. */
bool eys_fail(ey_rd *r, const char *why);
bool eys_tag(ey_rd *r, char want);
bool eys_read_uvarint(ey_rd *r, uint64_t *out);
bool eys_read_varint(ey_rd *r, int64_t *out);
bool eys_read_text(ey_rd *r, const char **s, int64_t *n);
bool eys_text_is(const char *s, int64_t n, const char *want);
bool eys_read_tagged(ey_rd *r, char tag, int64_t *out);
bool eys_read_int(ey_rd *r, ey_v *out);
bool eys_read_fixed(ey_rd *r, const ey_type *t, void *out);
bool eys_read_float(ey_rd *r, double *out);
bool eys_read_bool(ey_rd *r, bool *out);
bool eys_read_string(ey_rd *r, ey_v *out);
bool eys_read_bytes(ey_rd *r, ey_v *out);
bool eys_read_date(ey_rd *r, int64_t *days);
bool eys_read_time(ey_rd *r, int64_t *secs);
bool eys_read_instant(ey_rd *r, ey_v *out);
bool eys_read_list(ey_rd *r, const ey_type *lt, ey_decoder d, ey_v *out);
bool eys_read_set(ey_rd *r, const ey_type *st, ey_decoder d, ey_v *out);
bool eys_read_map(ey_rd *r, const ey_type *mt, ey_decoder kd, ey_decoder vd, ey_v *out);
bool eys_read_range(ey_rd *r, const ey_type *rt, ey_decoder d, ey_v *out);
bool eys_read_record(ey_rd *r, uint64_t *n); /* N or R, and how many fields follow */
bool eys_read_variant(ey_rd *r, const char **name, int64_t *len, uint64_t *n);

/* A binary built to check saving (cgen's tests): each module-level let of
 * the program's root module, by name, with its saved type. */
typedef struct ey_probe {
	const char *name;
	int saved;                 /* in ey_saved_types */
	const void *(*value)(void);
} ey_probe;
void ey_save_check(const ey_probe *ps, int n);

#endif
