cfg
===

Go library for working with app's configuration files used at Mailgun.

Usage
-----

Define a struct representing a configuration file. Optional fields may be marked with `config:"optional"` tag, so if they're missing from the config file, no error will occur; by default all fields are mandatory. Builtin structs are also supported:

```go
type Config struct {
  Keyspace string
  Servers  []string

  Builtin struct {
    Field string
  }

  PidPath string `config:"optional"`
}
```

Config file should be in YAML format and corresponding key names should match the ones in the defined struct:

```yaml
keyspace: blah
servers:
  - server1
  - server2

builtin:
  field: value
```

Load config into a struct:

```go
package main

import (
  "github.com/mailgun/cfg"
)

func main() {
  config := Config{}
  cfg.LoadConfig("path/to/config.yaml", &config)
}
```
