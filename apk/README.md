# Dagger `apk` Module

This is an attempt at implementing an apk container builder that uses merge-op to assemble the image by merging layers for each package. Basically what's described towards the end of this blog post: https://www.docker.com/blog/mergediff-building-dags-more-efficiently-and-elegantly/

Uses Chainguard's go-apk library to resolve a list of alpine packages into a DAG of packages. From there the Dagger API is used to download, unpack and merge packages.

# Status

- Rough draft but works
- Need to
  - turn into a real, customizable interface (just hard codes a lot of stuff)
  - support more repos such as wolfi
  - get "lazy remote rebasing" working fully
