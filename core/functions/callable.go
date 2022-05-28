package functions

type Callable interface {
	Enter()
	Command()
	Poll()

	MaxInputLength() int
}
