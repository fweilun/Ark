// README: Scheduled-order workflow placeholders (TODOs only for now).
package order

// TODO(schedule): Implement scheduled order flow described in docs/orderflow.md.
//
// TODO(schedule): Define status constants if needed:
// - StatusScheduled (Scheduled Order Created)
// - StatusAssigned (AcceptedWaiting / driver accepted, waiting for start)
// - StatusExpired (Expired)
//
// TODO(schedule): Define commands:
// - CreateScheduledCommand
// - ListScheduledByPassengerCommand
// - ListAvailableScheduledCommand
// - ClaimScheduledCommand
// - CancelScheduledCommand (passenger/driver variants)
//
// TODO(schedule): Service methods:
// - CreateScheduled
// - ListScheduledByPassenger
// - ListAvailableScheduled
// - ClaimScheduled
// - CancelScheduledByPassenger / CancelScheduledByDriver
// - RunScheduleIncentiveTicker
// - RunScheduleExpireTicker
//
// TODO(schedule): Store methods and SQL:
// - CreateScheduled
// - ListAvailableScheduled (by time window)
// - ClaimScheduled (optimistic lock)
// - CancelScheduledByPassenger
// - ReopenScheduled (driver cancel -> back to scheduled)
