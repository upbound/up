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
1. **branch repo**: Create a new release branch using the GitHub UI for the
   repo (e.g. `release-0.25`).
1. **tag release**: Run the `Tag` action on the _release branch_ with the
   desired version (e.g. `v0.25.0`).
1. **build/publish**: Run the `CI` action on the release-branch (**not on the tag!**).
1. **tag next pre-release**: Run the `tag` action on the main development branch
   with the `rc.0` for the next release (e.g. `v0.26.0-rc.0`).
1. **verify**: Verify all artifacts have been published successfully, perform
   sanity testing.
   - Check in https://cli.upbound.io/stable?prefix=build/release-0.25/v0.25.0.
     Download some binaries / package formats and smoke test them, e.g. by 
     - (all platforms) download your architecture from the `bin` folder and run 
       it: `up version`.
     - TODO: add more here
   - **note**: You may keep downloading the old version for a while until CDN
     cache is refreshed.
1. **promote**: Run the `Promote` action on the release version to promote 
   to desired channel(s) (e.g. `alpha` or `stable`).
1. **update homebrew**: Run [`Bump Formula`](https://github.com/upbound/homebrew-tap/actions/workflows/bump-formula.yaml) action to open a PR in Homebrew 
   for the new version. Get approval and merge.
1. **release notes**: 
   - Open the new release tag in https://github.com/upbound/up/tags and click "Create
     release from tag".
   - "Generate release notes" from previous release ("auto" might not work).
   - Make sure the release notes are complete, presize and well formatted.
   - Publish the well authored Github release.
1. **wait for CDN**: Wait for CloudFront to distribute the artifacts, e.g. wait
   until `curl -sL https://cli.upbound.io | sh -x && ./up version` gives the new
   release.
1. **announce**: Announce the release on Twitter, Slack, etc.
   - Crossplane Slack #Upbound: https://crossplane.slack.com/archives/C01TRKD4623
   - TODO: where else?