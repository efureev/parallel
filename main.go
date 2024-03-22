package main

import (
	parallel "github.com/efureev/parallel/src"
)

func main() {

	flags := parallel.ParseFlag()

	lgr := parallel.Logger()

	loader := parallel.NewFileLoader(parallel.YamlFileMarshaller{}, lgr)
	flow := loader.Load(flags.ConfigFilePath)

	mgr := parallel.Manager(lgr)

	mgr.RunParallel(flow.Chains)

	lgr.Debug().Msg(`App Finished`)
}
