/* Values at the program's boundary: JSON (json.c), saved values (save.c)
 * and calls of host functions the Go runner answers (host.c). */
#ifndef EYE_DATA_H
#define EYE_DATA_H

#include "eye.h"

/* toJson of the value of type t at p (SPECS.md §17, "Values as JSON") */
ey_v ey_to_json(const char *at, const ey_type *t, const void *p);

#endif
