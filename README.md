# Parallel

[![Go Coverage](https://github.com/efureev/parallel/wiki/coverage.svg)](https://raw.githack.com/wiki/efureev/reggol/coverage.html)

## Install

```bash
go install github.com/efureev/parallel
```

## Run

If you have a flow-file `.parallelrc.yaml` in a folder of execution:

```bash
parallel
```

Of a flow-file has different destination:

```bash
parallel -f /...../app/flow.yaml
```

## Structure of FlowFile

Lang: `yaml`

```yaml
commands: # list of parallel commands
  php serve: # name of a Command Chain
    artisan: # name of Command
      pipe: true  # listen stdOutput & stdErr for this command
      cmd: [ 'php', 'artisan', '--port', '8010' ] # One Command and its args 
      dir: 'app' # Directory of an execution

  yarn dev:
    ls: # This command will be executed without Pipe
      cmd: [ 'ls', '-la' ]
    yarn:
      pipe: true
      cmd: [ 'yarn', 'dev' ]
      dir: 'app'
  net:
    ping:
      pipe: true
      cmd: [ 'ping', '-c', '3','ya.ru' ]
```
