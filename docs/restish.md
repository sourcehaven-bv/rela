<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Driving rela-server from the command line with restish

[restish](https://rest.sh) is a generic command-line client for any API that
publishes an OpenAPI description. rela-server publishes one for its project
schema at `/api/v1/_openapi.json`, so restish can call a remote server with
one command per operation: `list-tasks`, `get-task`, `update-task`,
`put-task-attachment`, and so on. It handles the OAuth sign-in itself.

The main use is a script or AI agent that needs to send a file to an entity.
The agent passes the file to restish by path, and the bytes go straight from
disk to the server. They never pass through the model as base64.

## What the spec contains

The spec is generated from `schema.yaml` and changes when the schema is
reloaded. Restish caches it; run `restish api sync <name>` after a schema
change.

- One set of operations per entity type: list, create, get, update, delete,
  clone, and the relation operations.
- For a type with `file` properties: upload, download and delete of an
  attachment. The upload takes the file as the raw request body, with the
  name in `--filename`. See
  [the upload endpoint](data-entry/api-reference.md#upload-endpoint).
- A security scheme for the header the server reads its identity from (the
  `-jwt-header` flag). With `Authorization` it is a bearer token.
- Its `servers` entry is `/`: the paths are absolute, so the base URL is the
  origin of the server (or of the proxy in front of it).

The spec describes the configuration, which is not secret. It is still
served only to an authenticated caller, like every `/api/` path.

## Setup behind an OAuth proxy

A production rela-server sits behind an authenticating proxy that turns the
caller's OAuth access token into the signed assertion rela verifies. The
example uses Pratique, which accepts a bearer token on any path. The steps
are the same for any proxy that does this.

1. Register a client with the proxy. Pratique supports dynamic client
   registration; the redirect URI must be the one restish listens on:

   ```sh
   curl -s https://rela.example/__pratique/oauth/register \
     -H 'Content-Type: application/json' \
     -d '{"client_name":"restish","redirect_uris":["http://localhost:8484/"]}'
   ```

   Keep the returned `client_id`. Sign in within an hour, or Pratique
   discards the unclaimed registration.

2. Add the API to restish's configuration (`~/.config/restish/restish.json`
   on Linux; run `restish api edit` to open it):

   ```json
   {
     "rela": {
       "base_url": "https://rela.example",
       "spec_files": ["https://rela.example/api/v1/_openapi.json"],
       "profiles": {
         "default": {
           "auth": {
             "name": "oauth-authorization-code",
             "params": {
               "client_id": "dcr_…",
               "authorize_url": "https://rela.example/__pratique/oauth/authorize",
               "token_url": "https://rela.example/__pratique/oauth/token"
             }
           }
         }
       }
     }
   }
   ```

   Give the spec URL explicitly. restish looks for a spec at the root of the
   base URL, and rela serves it under `/api/v1/`.

3. Sign in and load the spec. The first call opens the browser for the OAuth
   sign-in; restish then caches the token and refreshes it.

   ```sh
   restish api sync rela
   restish rela list-tasks
   ```

## Uploading and downloading a file

```sh
restish rela put-task-attachment TASK-1 evidence --filename shot.png < shot.png
restish rela get-task-attachment TASK-1 evidence shot.png > copy.png
```

The upload answers with the updated entity. Its `_attachments` entry holds
the stored name, which can differ from `--filename`: on a property that
holds several files, a name already in use gets a suffix such as
`shot (1).png`. restish reads standard input into memory and caps it at
16 MiB, which is below the server's default limit of 64 MiB.

## Without a proxy

A rela-server that verifies its own JWTs (`-jwt-*` flags with
`-jwt-header Authorization`) accepts any bearer token its issuer signed. Use
whichever restish auth type matches that issuer. A server without a JWT gate
publishes no security scheme, and restish calls it without credentials.

restish sends no `Origin`, `Referer` or cookie, so the server's
same-origin check lets it through on the paths a non-browser client uses:
the entity routes, `_schema` and `_openapi.json`. See
[server security](server-security.md#calling-the-api-from-curl-scripts-or-non-browser-clients).
