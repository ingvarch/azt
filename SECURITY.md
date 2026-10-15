# Security policy

## Supported versions

Only the latest release gets fixes. Check that the problem is still there in
it before you report.

## Reporting a vulnerability

Do not open a public issue. Report it privately on the
[Security tab](https://github.com/ingvarch/azt/security/advisories/new): only
the maintainer sees the report. Say what an attacker can do and how to
reproduce it.

The fix is agreed on in the report. The advisory is published with the release
that fixes it.

## What azt can reach

azt reads Azure through Azure Resource Graph with the account you signed in to
with `az login`. It does not create, change or delete resources, and it stores
no credentials. It writes one file, the layout of the last session, to
`$XDG_STATE_HOME/azt/session.json` (by default
`~/.local/state/azt/session.json`) with mode 0600. The file holds subscription
IDs, resource group names and resource IDs, and no data from the resources.

## Verifying a release

Every release publishes `checksums.txt` with a keyless
[cosign](https://github.com/sigstore/cosign) signature in
`checksums.txt.sigstore.json`. It names the release workflow of this
repository. To check a download:

```sh
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/ingvarch/azt/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt
```

The macOS binaries are signed and notarized by Apple. Each archive also comes
with an SBOM of the modules it is built from.
