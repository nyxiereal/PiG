package session

import "github.com/MichaelKinsy/PiG/chord"

// NewSessionWatch returns a Chord Watch whose deliveries run on the Session's delivery scheduler and whose listener
// failures go to the Session's report, as Pi's Durable builds each Watch with (error) => session.report(error)
// (session.ts #attachDocument, view.ts, task-graph.ts). detach runs once when the watch ends.
func NewSessionWatch[T any](session *SessionImpl, value T, detach func()) *chord.Watch[T] {
	return chord.NewWatch(value, detach, session.reportListener).On(session.scheduler)
}

// NewSessionNativeWatch is NewSessionWatch for a native value (events.ts): every value is delivered in order, one call each;
// replace gives the value an overflow delivers, and nil delivers the newest value.
func NewSessionNativeWatch[T any](session *SessionImpl, value T, detach func(), replace func() T) *chord.NativeWatch[T] {
	return chord.NewNativeWatch(value, detach, session.reportListener, replace).On(session.scheduler)
}
