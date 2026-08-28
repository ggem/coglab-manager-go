// Package scheduling finds valid date/times to run an experiment session,
// given lab members' availability, equipment capacity, and role
// qualifications. It's a from-scratch port of the legacy BRL app's
// bitvector-grid + backtracking search
// (appointments:schedule:find-availability / assign-roles), kept
// deliberately free of SQL and HTTP concerns: callers assemble plain Go
// inputs from whatever storage they use and get plain Go results back.
//
// Two phases, matching the legacy design:
//
//  1. A coarse per-day grid filter (MergeRoleAvailability, IntersectAll,
//     TrimForDuration) that's cheap to compute and tells you which time
//     slots are even worth trying -- a necessary condition, not a final
//     answer, since "someone qualified for each role is free" doesn't
//     guarantee a single consistent assignment across all roles at once.
//  2. An exact backtracking search (FindAssignment), run only on slots
//     that survive phase 1, that finds one real simultaneous assignment
//     or proves none exists.
//
// The legacy engine encodes phase 1 with packed integer "bitvectors" and
// phase 2 with an explicit Scheme failure-continuation (CPS-style
// backtracking). This package uses plain []bool-backed Slots (168 slots
// is not a scale where bit-packing buys anything, and []bool reads far
// more clearly) and ordinary recursive backtracking with early returns
// (Go has no natural equivalent of CPS, and forcing one would be
// unreadable) -- same algorithm and search order, different idiom.
package scheduling
