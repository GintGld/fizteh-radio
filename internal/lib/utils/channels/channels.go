package chans

func Notify[T any](ch chan<- T, s T) {
	if ch != nil {
		select {
		case ch <- s:
		default:
		}
	}
}
