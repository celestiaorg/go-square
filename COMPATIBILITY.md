# Compatibility

This document describes version compatibility between go-square, celestia-app, and share versions.

## Compatibility Matrix

| celestia-app | App Version | go-square | Share Versions |
|---|---|---|---|
| v2.x | 2 | v1.1.0 | v0 |
| v3.x | 3 | v1.1.1, v2.3.0 | v0, v1 |
| v4.x | 4 | v2.3.1 | v0, v1 |
| v5.x | 5 | v2.3.1 | v0, v1 |
| v6.x | 6 | v2.3.3, v3.0.2 | v0, v1 |
| v7.x | 7 | v2.3.3, v3.0.2 | v0, v1 |
| main | 8 | v2.3.3, v4.0.0-rc3 | v0, v1, v2 |

## Share Versions

- **v0**: Original format (blobs without signer).
- **v1**: Adds signer field (authored blobs). Introduced in go-square v2.
- **v2**: Adds Fibre blob version and commitment (Fibre system blobs). Introduced in go-square v4.
