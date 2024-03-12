# Contributing

For styling guidelines, see [this document](https://github.com/crossplane/crossplane/blob/master/contributing/README.md).

## Development Environment

The project includes a git submodule that includes various helpers. After
cloning, you'll need to make `git` download it with the following command.

```bash
make submodules
```

The rest is just a usual Golang CLI project where you can find the executables
under `cmd` folder.

## Release Process

This is a slimmed-down version of the release process described [here](https://github.com/crossplane/release).

1. **feature freeze**: Merge all completed features into main development branch
   of all repos to begin "feature freeze" period.
1. **pin dependencies**: Update the go module on main development branch to
   depend on stable versions of dependencies if needed.
1. **branch repo**: Create a new release branch using the GitHub UI for the
   repo.
1. **tag release**: Run the `Tag` action on the _release branch_ with the
   desired version (e.g. `v0.14.0`).
1. **build/publish**: Run the `CI` action on the release branch with the version
   that was just tagged.
1. **tag next pre-release**: Run the `tag` action on the main development branch
   with the `rc.0` for the next release (e.g. `v0.15.0-rc.0`).
1. **verify**: Verify all artifacts have been published successfully, perform
   sanity testing.
     **note**: You may keep downloading the old version for a while until CDN
     cache is refreshed.
1. **promote**: Run the `Promote` action to promote release to desired
   channel(s).
     **update homebrew**: Run [`Bump Formula`](https://github.com/upbound/homebrew-tap/actions/workflows/bump-formula.yaml) action to open a PR in Homebrew for
     the new version.
1. **release notes**: Publish well authored and complete release notes on
   GitHub.
1. **announce**: Announce the release on Twitter, Slack, etc.
