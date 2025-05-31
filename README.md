# Parallel

[![Go Coverage](https://github.com/efureev/parallel/wiki/coverage.svg)](https://raw.githack.com/wiki/efureev/reggol/coverage.html)
[![Test](https://github.com/efureev/parallel/actions/workflows/test.yml/badge.svg)](https://github.com/efureev/parallel/actions/workflows/test.yml)

A tool for running multiple console commands in parallel with output display in the terminal.

## Installation

```shell script
go install github.com/efureev/parallel@latest
```


## Usage

If you have a configuration file `.parallelrc.yaml` in the execution folder:

```shell script
parallel
```


If the configuration file is located elsewhere:

```shell script
parallel -f /path/to/config/flow.yaml
```


## Screenshots

![screen1.png](.assets%2Fscreen1.png)
![sceen2.png](.assets%2Fsceen2.png)
![screen3.png](.assets%2Fscreen3.png)

## Configuration File Structure

Language: `yaml`

```yaml
commands: # list of parallel commands
  php-server: # command chain name
    artisan: # command name
      pipe: true  # listen to stdOutput & stdErr for this command
      cmd: [ 'php', 'artisan', 'serve', '--port', '8010' ] # command and its arguments
      dir: 'app' # execution directory

  web-services: # command mode
    nginx-cmd:
      pipe: true
      cmd: [ 'docker', 'container', 'run',  '--rm', '-p', '8090:80', '--name', 'nginx', 'nginx' ]
      format:
        cmdName: '%CMD_NAME% %CMD_ARGS%' # command name formatting

  docker-services: # Docker mode
    nginx-docker:
      docker:
        image:
          name: 'nginx'
          # tag: 'v1' # default 'latest'
          # pull: 'always' # default: none
        ports: [
          '127.0.0.1:80:8080',
          '127.0.0.1:443:8443',
        ]
        # removeAfterAll: false # default: true
        # cmd: 'exec' # default: 'run'

  frontend:
    list-files: # this command will execute without pipe
      cmd: [ 'ls', '-la' ]
    yarn-dev:
      pipe: true
      cmd: [ 'yarn', 'dev' ]
      dir: 'app'

  network:
    ping-test:
      pipe: true
      cmd: [ 'ping', '-c', '3','ya.ru' ]
```


## Features

- **Command Chains**: Group related commands into logical blocks for better organization
- **Docker Support**: Built-in support for running Docker containers with automatic command formatting
- **Colored Output**: Each command chain gets a unique color for better readability
- **Output Control**: Control which commands should display their output via the `pipe` parameter
- **Working Directories**: Specify custom directories for command execution
- **Command Formatting**: Customize how command names are displayed in the output
- **Auto-cleanup**: Docker containers are automatically removed after execution (configurable)
- **Flexible Configuration**: Support for both direct commands and Docker-based workflows