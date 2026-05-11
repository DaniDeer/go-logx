module github.com/DaniDeer/go-logx/examples/pkg-errors

go 1.26.2

require (
	github.com/DaniDeer/go-logx v0.0.0
	github.com/pkg/errors v0.9.1
)

require gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect

replace github.com/DaniDeer/go-logx => ../..
