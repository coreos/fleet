# fleet release guide

The guide talks about how to release a new version of fleet.

The procedure includes some manual steps for sanity checking but it can probably be further scripted. Please keep this document up-to-date if you want to make changes to the release process.

## Prepare Release

Set desired version as environment variable for following steps. Here is an example to release 0.13.0:

```
export VERSION=v0.13.0
```

All releases version numbers follow the format of [semantic versioning 2.0.0](http://semver.org/).

### Major, Minor Version Release, or its Pre-release

- Ensure the relevant milestone on GitHub is complete. All referenced issues should be closed, or moved elsewhere.
- Ensure the latest upgrade documentation is available.
- Add feature capability maps for the new version, if necessary.

## Write Release Note

- Write introduction for the new release. For example, what major bug we fix, what new features we introduce or what performance improvement we make.
- Write changelog for the last release. The changelog should be straightforward and easy to understand for the end-user.

## Tag Version

- Ensure all tests on CI system are passed.
- Manually check fleet is buildable in Linux, Darwin.
- Manually check upgrade fleet cluster of previous minor version works well.
- Manually check new features work well.
- Add an annotated tag through `git tag -a ${VERSION}`.
- Sanity check tag correctness through `git show tags/$VERSION`.
- Push the tag to GitHub through `git push origin tags/$VERSION`. This assumes `origin` corresponds to "https://github.com/coreos/fleet".

## Build Release Binaries and Images

- Ensure `acbuild` is available.
- Ensure `docker` is available.

Run release script in root directory:

```
./scripts/release.sh ${VERSION}
```

It generates all release binaries and images under directory `./release`.

## Publish Release Page in GitHub

- Set release title as the version name.
- Follow the format of previous release pages.
- Attach the generated binaries and aci image.
- Publish the release!

## Publish Docker Image in Quay.io

- Push docker image:

```
docker login quay.io
docker push quay.io/coreos/fleet:${VERSION}
```

- Add `latest` tag to the new image on [quay.io](https://quay.io/repository/coreos/fleet?tag=latest&tab=tags) if this is a stable release.
