---
title: Guide
---

# Guide

The guide explains one idea per page: what it is for, how to use it well, and the mistakes it
prevents. The [tutorial](/tutorial/) walks through the language in order; the
[reference](/reference/language/) states the exact rules. Every guide's code is a real program that
the test suite runs.

## State and waiting

- [Entities](/guide/entities): state with one owner, changed one transaction at a time.
- [Jobs and waiting](/guide/jobs-and-waiting): durable functions that pause at `!` steps for as long
  as they need and survive restarts.
- [Views, events and schedules](/guide/views-events-schedules): starting jobs when entities change,
  when something happens, or at a time of day.
- [Testing with time and recordings](/guide/testing): fresh stores, a clock you move, and services
  replayed from recordings.

## Boundaries

- [Talking to other services](/guide/contracts): contracts, keys, lost replies, and the wire format.
- [Effects](/guide/effects): what a function may touch, stated where it matters.
- [AI calls](/guide/ai-calls): asking a model for a typed value.

## Shipping

- [Running in production](/guide/running): `eye run`, the journal, `eye runs`, and configuration.
- [Doc comments](/guide/doc-comments): documenting code for people, editors and models.

The basics (values, mutation, types and matching, errors) are taught by the [tutorial](/tutorial/)
and specified in the [language reference](/reference/language/).
