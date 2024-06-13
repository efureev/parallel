# Parallel

[![Go Coverage](https://github.com/efureev/parallel/wiki/coverage.svg)](https://raw.githack.com/wiki/efureev/reggol/coverage.html)

Allows you to run several console commands in parallel and output its output in your term.

## Install

```bash
go install github.com/efureev/parallel@latest
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

## Screens

![screen1.png](.assets%2Fscreen1.png)
![sceen2.png](.assets%2Fsceen2.png)
![screen3.png](.assets%2Fscreen3.png)

## Structure of FlowFile

Lang: `yaml`

```yaml
commands: # list of parallel commands
  php serve: # name of a Command Chain
    artisan: # name of Command
      pipe: true  # listen stdOutput & stdErr for this command
      cmd: [ 'php', 'artisan', '--port', '8010' ] # One Command and its args 
      dir: 'app' # Directory of an execution

  nginx:
    docker:
      pipe: true
      cmd: [ 'docker', 'container', 'run',  '--rm', '-p', '8090:80', '--name', 'ngixn', 'nginx' ]
      format:
        cmdName: '%CMD_NAME% %CMD_ARGS%' # command name formatting 

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
