module example.com/workspace/moduleB

go 1.25.0

require (
	example.com/workspace/moduleA v0.0.0
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/text v0.35.0 // indirect
)

replace example.com/workspace/moduleA => ../moduleA
