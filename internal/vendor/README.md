## vendor
This directory is the unfortunate consequence of some very handy golang/tools 
APIs for working with the LSP protocol being hidden from imports behind an
`internal` package directory.

Rather than importing more than we need, the directory currently contains the
packages that we find useful for processing line spans coming from the LSP
client.

> Note: the structure currently follows what you'll find in https://github.com/golang/tools
>       with the exception being that `internal` is removed from the directory structure.