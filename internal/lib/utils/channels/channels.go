package chans

// Notify send an empty struct
// to a channel.
//
// If channel is nil, does nothing.
func Notify(ch chan<- struct{}) {
	if ch != nil {
		go func() {
			ch <- struct{}{}
		}()
	}
}

// Send sends an object to a channel.
//
// If channel is nil, does nothing.
func Send[T any](ch chan<- T, s T) {
	if ch != nil {
		go func() {
			ch <- s
		}()
	}
}
