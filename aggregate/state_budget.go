// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"fmt"
	"maps"
	"sync"
)

// StateKind identifies a retained aggregate or tail-sampling state type.
// It deliberately does not contain a workspace or group identifier, so callers
// can use it directly as a low-cardinality metric dimension.
type StateKind string

const (
	// StateKindTailSamplingConfig accounts for initialized tail-sampling configuration.
	StateKindTailSamplingConfig StateKind = "tail_sampling_config"
	// StateKindTailSamplingGroup accounts for one active tail-sampling group.
	StateKindTailSamplingGroup StateKind = "tail_sampling_group"
	// StateKindTailSamplingPayload accounts for payload bytes retained by active groups.
	StateKindTailSamplingPayload StateKind = "tail_sampling_payload"
	// StateKindTailSamplingTombstone accounts for a dropped group marker kept until its TTL expires.
	StateKindTailSamplingTombstone StateKind = "tail_sampling_tombstone"
	// StateKindAggregationBucket accounts for one active aggregation expiration bucket.
	StateKindAggregationBucket StateKind = "aggregation_bucket"
	// StateKindAggregationWindow accounts for one workspace window in an expiration bucket.
	StateKindAggregationWindow StateKind = "aggregation_window"
	// StateKindAggregationCalculator accounts for one retained aggregation calculator.
	StateKindAggregationCalculator StateKind = "aggregation_calculator"
	// StateKindAggregationPoint accounts for input points retained in an aggregation window's result state.
	StateKindAggregationPoint StateKind = "aggregation_point"
)

// StateCost is the bounded resource cost of retained state. A zero limit means
// unlimited, while zero cost is useful for tracking an item-only budget.
type StateCost struct {
	Bytes   int64
	Objects int64
}

// Add returns the combined state cost.
func (c StateCost) Add(other StateCost) StateCost {
	return StateCost{Bytes: c.Bytes + other.Bytes, Objects: c.Objects + other.Objects}
}

// Sub returns the remaining state cost and never returns negative values.
func (c StateCost) Sub(other StateCost) StateCost {
	return StateCost{
		Bytes:   maxInt64(0, c.Bytes-other.Bytes),
		Objects: maxInt64(0, c.Objects-other.Objects),
	}
}

// StateLimit bounds aggregate usage and an individual lease independently. A
// non-positive value is unlimited. Per-reservation limits are what let callers
// protect one trace group or one aggregation window without converting a
// workspace protection limit into an unfair per-tenant minimum.
type StateLimit struct {
	MaxBytes                 int64
	MaxObjects               int64
	MaxBytesPerReservation   int64
	MaxObjectsPerReservation int64
}

// StateBudgetMode decides whether excess state is admitted for observation or rejected.
type StateBudgetMode string

const (
	// StateBudgetObserve keeps compatibility behavior and only records a would-reject event.
	StateBudgetObserve StateBudgetMode = "observe"
	// StateBudgetEnforce rejects a reservation that would exceed a configured limit.
	StateBudgetEnforce StateBudgetMode = "enforce"
)

// StateBudgetConfig configures process, workspace, and state-kind resource limits.
// A workspace is supplied by callers as a token or another stable tenant identity;
// this module never exports it in a snapshot.
type StateBudgetConfig struct {
	Mode               StateBudgetMode
	Process            StateLimit
	Workspace          StateLimit
	Kinds              map[StateKind]StateLimit
	WorkspaceKindLimit map[StateKind]StateLimit
}

// StateReservation describes one retained state allocation.
type StateReservation struct {
	Workspace string
	Kind      StateKind
	Cost      StateCost
}

// StateBudgetScope identifies the limit that rejected an allocation.
type StateBudgetScope string

const (
	StateBudgetScopeProcess       StateBudgetScope = "process"
	StateBudgetScopeWorkspace     StateBudgetScope = "workspace"
	StateBudgetScopeKind          StateBudgetScope = "kind"
	StateBudgetScopeWorkspaceKind StateBudgetScope = "workspace_kind"
	StateBudgetScopeObject        StateBudgetScope = "object"
	StateBudgetScopeReleasedLease StateBudgetScope = "released_lease"
)

// StateBudgetError provides enough context for a transport layer to map a
// resource rejection to its protocol-specific status without exposing a token.
type StateBudgetError struct {
	Scope       StateBudgetScope
	Reservation StateReservation
	Limit       StateLimit
	Current     StateCost
	Requested   StateCost
}

func (e *StateBudgetError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("state budget rejected %s at %s limit", e.Reservation.Kind, e.Scope)
}

// StateUsage contains aggregate state usage without high-cardinality identifiers.
type StateUsage struct {
	Cost          StateCost
	Rejected      uint64
	WouldRejected uint64
}

// StateBudgetSnapshot is safe to expose to metrics collectors. Workspace usage
// remains intentionally internal because workspace identifiers are high cardinality.
type StateBudgetSnapshot struct {
	Total  StateUsage
	ByKind map[StateKind]StateUsage
}

// StateLease owns a state reservation until Release is called. Resize changes a
// retained allocation in place, which is needed for payload append and bounded
// calculator growth without allocating a lease per input point.
type StateLease struct {
	reservation StateReservation
	budget      *stateBudget
	active      bool
}

// Reservation returns the current reservation associated with the lease.
func (l *StateLease) Reservation() StateReservation {
	if l == nil {
		return StateReservation{}
	}
	return l.reservation
}

// StateBudget is the shared state-admission seam used by tail sampling and aggregation.
type StateBudget interface {
	Reserve(StateReservation) (*StateLease, *StateBudgetError)
	Resize(*StateLease, StateCost) *StateBudgetError
	Release(*StateLease)
	Snapshot() StateBudgetSnapshot
}

type stateBudget struct {
	lock sync.Mutex
	cfg  StateBudgetConfig

	total        StateUsage
	byKind       map[StateKind]StateUsage
	byWorkspace  map[string]StateCost
	byWorkspaceK map[string]map[StateKind]StateCost
}

// NewStateBudget creates a concurrency-safe state budget. A nil budget is also
// accepted by the aggregate constructors and means legacy unlimited behavior.
func NewStateBudget(cfg StateBudgetConfig) StateBudget {
	if cfg.Mode != StateBudgetEnforce {
		cfg.Mode = StateBudgetObserve
	}
	return &stateBudget{
		cfg:          cfg,
		byKind:       make(map[StateKind]StateUsage),
		byWorkspace:  make(map[string]StateCost),
		byWorkspaceK: make(map[string]map[StateKind]StateCost),
	}
}

func (b *stateBudget) Reserve(reservation StateReservation) (*StateLease, *StateBudgetError) {
	if b == nil {
		return &StateLease{reservation: reservation, active: true}, nil
	}
	reservation.Cost = normalizeStateCost(reservation.Cost)
	b.lock.Lock()
	defer b.lock.Unlock()

	if rejection := b.rejectionLocked(reservation, reservation.Cost, reservation.Cost); rejection != nil {
		b.recordRejectionLocked(reservation.Kind)
		if b.cfg.Mode == StateBudgetEnforce {
			return nil, rejection
		}
	}
	b.addUsageLocked(reservation, reservation.Cost)
	return &StateLease{reservation: reservation, budget: b, active: true}, nil
}

func (b *stateBudget) Resize(lease *StateLease, cost StateCost) *StateBudgetError {
	if lease == nil {
		return &StateBudgetError{Scope: StateBudgetScopeReleasedLease}
	}
	if lease.budget == nil {
		lease.reservation.Cost = normalizeStateCost(cost)
		return nil
	}
	if lease.budget != b {
		return &StateBudgetError{Scope: StateBudgetScopeReleasedLease, Reservation: lease.reservation}
	}

	cost = normalizeStateCost(cost)
	b.lock.Lock()
	defer b.lock.Unlock()
	if !lease.active {
		return &StateBudgetError{Scope: StateBudgetScopeReleasedLease, Reservation: lease.reservation}
	}

	delta := StateCost{Bytes: cost.Bytes - lease.reservation.Cost.Bytes, Objects: cost.Objects - lease.reservation.Cost.Objects}
	if delta.Bytes > 0 || delta.Objects > 0 {
		if rejection := b.rejectionLocked(lease.reservation, delta, cost); rejection != nil {
			b.recordRejectionLocked(lease.reservation.Kind)
			if b.cfg.Mode == StateBudgetEnforce {
				return rejection
			}
		}
	}
	b.addUsageLocked(lease.reservation, delta)
	lease.reservation.Cost = cost
	return nil
}

func (b *stateBudget) Release(lease *StateLease) {
	if lease == nil || lease.budget != b {
		return
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	if !lease.active {
		return
	}
	b.addUsageLocked(lease.reservation, StateCost{Bytes: -lease.reservation.Cost.Bytes, Objects: -lease.reservation.Cost.Objects})
	lease.active = false
}

func (b *stateBudget) Snapshot() StateBudgetSnapshot {
	if b == nil {
		return StateBudgetSnapshot{ByKind: map[StateKind]StateUsage{}}
	}
	b.lock.Lock()
	defer b.lock.Unlock()

	byKind := make(map[StateKind]StateUsage, len(b.byKind))
	maps.Copy(byKind, b.byKind)
	return StateBudgetSnapshot{Total: b.total, ByKind: byKind}
}

func (b *stateBudget) rejectionLocked(reservation StateReservation, delta, requested StateCost) *StateBudgetError {
	requested = normalizeStateCost(requested)
	if err := checkReservationLimit(b.cfg.Kinds[reservation.Kind], requested); err != nil {
		return stateBudgetError(err, reservation, requested)
	}
	if err := checkReservationLimit(b.cfg.WorkspaceKindLimit[reservation.Kind], requested); err != nil {
		return stateBudgetError(err, reservation, requested)
	}
	if err := checkStateLimit(StateBudgetScopeProcess, b.cfg.Process, b.total.Cost, delta); err != nil {
		return stateBudgetError(err, reservation, requested)
	}

	workspaceUsage := b.byWorkspace[reservation.Workspace]
	if err := checkStateLimit(StateBudgetScopeWorkspace, b.cfg.Workspace, workspaceUsage, delta); err != nil {
		return stateBudgetError(err, reservation, requested)
	}

	kindUsage := b.byKind[reservation.Kind].Cost
	if err := checkStateLimit(StateBudgetScopeKind, b.cfg.Kinds[reservation.Kind], kindUsage, delta); err != nil {
		return stateBudgetError(err, reservation, requested)
	}

	workspaceKindUsage := b.byWorkspaceK[reservation.Workspace][reservation.Kind]
	if err := checkStateLimit(StateBudgetScopeWorkspaceKind, b.cfg.WorkspaceKindLimit[reservation.Kind], workspaceKindUsage, delta); err != nil {
		return stateBudgetError(err, reservation, requested)
	}
	return nil
}

type stateLimitError struct {
	scope   StateBudgetScope
	limit   StateLimit
	current StateCost
}

func checkStateLimit(scope StateBudgetScope, limit StateLimit, current, delta StateCost) *stateLimitError {
	if limit.MaxBytes > 0 && current.Bytes+delta.Bytes > limit.MaxBytes {
		return &stateLimitError{scope: scope, limit: limit, current: current}
	}
	if limit.MaxObjects > 0 && current.Objects+delta.Objects > limit.MaxObjects {
		return &stateLimitError{scope: scope, limit: limit, current: current}
	}
	return nil
}

func checkReservationLimit(limit StateLimit, requested StateCost) *stateLimitError {
	if limit.MaxBytesPerReservation > 0 && requested.Bytes > limit.MaxBytesPerReservation {
		return &stateLimitError{scope: StateBudgetScopeObject, limit: limit}
	}
	if limit.MaxObjectsPerReservation > 0 && requested.Objects > limit.MaxObjectsPerReservation {
		return &stateLimitError{scope: StateBudgetScopeObject, limit: limit}
	}
	return nil
}

func stateBudgetError(limitErr *stateLimitError, reservation StateReservation, requested StateCost) *StateBudgetError {
	return &StateBudgetError{
		Scope:       limitErr.scope,
		Reservation: reservation,
		Limit:       limitErr.limit,
		Current:     limitErr.current,
		Requested:   requested,
	}
}

func (b *stateBudget) recordRejectionLocked(kind StateKind) {
	usage := b.byKind[kind]
	if b.cfg.Mode == StateBudgetEnforce {
		usage.Rejected++
		b.total.Rejected++
	} else {
		usage.WouldRejected++
		b.total.WouldRejected++
	}
	b.byKind[kind] = usage
}

func (b *stateBudget) addUsageLocked(reservation StateReservation, delta StateCost) {
	b.total.Cost = addSignedCost(b.total.Cost, delta)

	kindUsage := b.byKind[reservation.Kind]
	kindUsage.Cost = addSignedCost(kindUsage.Cost, delta)
	b.byKind[reservation.Kind] = kindUsage

	workspaceUsage := addSignedCost(b.byWorkspace[reservation.Workspace], delta)
	if workspaceUsage == (StateCost{}) {
		delete(b.byWorkspace, reservation.Workspace)
	} else {
		b.byWorkspace[reservation.Workspace] = workspaceUsage
	}

	workspaceKinds := b.byWorkspaceK[reservation.Workspace]
	if workspaceKinds == nil {
		workspaceKinds = make(map[StateKind]StateCost)
		b.byWorkspaceK[reservation.Workspace] = workspaceKinds
	}
	workspaceKindUsage := addSignedCost(workspaceKinds[reservation.Kind], delta)
	if workspaceKindUsage == (StateCost{}) {
		delete(workspaceKinds, reservation.Kind)
		if len(workspaceKinds) == 0 {
			delete(b.byWorkspaceK, reservation.Workspace)
		}
	} else {
		workspaceKinds[reservation.Kind] = workspaceKindUsage
	}
}

func normalizeStateCost(cost StateCost) StateCost {
	return StateCost{Bytes: maxInt64(0, cost.Bytes), Objects: maxInt64(0, cost.Objects)}
}

func addSignedCost(current, delta StateCost) StateCost {
	return StateCost{
		Bytes:   maxInt64(0, current.Bytes+delta.Bytes),
		Objects: maxInt64(0, current.Objects+delta.Objects),
	}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
