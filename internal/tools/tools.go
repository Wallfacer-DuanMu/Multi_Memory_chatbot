package tools

type Tool interface {
	Name() string
	Execute(input any) (any, error)
}
