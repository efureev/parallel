package main

import (
	"context"
	"os/signal"
	"syscall"

	parallel "github.com/efureev/parallel/src"
)

func main() {
	flags := parallel.ParseFlag()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	lgr := parallel.Logger()

	loader := parallel.NewFileLoader(parallel.YamlFileMarshaller{}, lgr)
	flow := loader.Load(flags.ConfigFilePath)

	mgr := parallel.Manager(lgr)

	mgr.RunParallel(ctx, flow.Chains)

	<-ctx.Done()

	lgr.Debug().Msg(`App Finished`)
}
