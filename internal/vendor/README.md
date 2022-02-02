## vendor
This directory is the unfortunate consequence of some very handy golang/tools 
APIs for working with the LSP protocol as well as some handy APIs for working
with crossplane resources being hidden from imports behind an `internal` 
package directory.

Rather than importing more than we need, the directory currently contains the
packages that we find useful for processing line spans coming from the LSP
client as well as some of the CompositionRevision reconciler logic from 
crossplane.

> Note: the structure currently follows what you'll find in https://github.com/golang/tools
>       and https://github.com/crossplane/crossplane with the exception being that `internal` 
>       is removed from the directory structure.
