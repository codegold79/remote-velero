# Changes

- Enable multi-namespace support for recognizing remote cluster service account information.
- Remove service account name and namespace from secrets. Rename "sa-token" key to "serviceaccount".
- Add support for "remotecluster" secret that will serve as both source and destination cluster credentials.
