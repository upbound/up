# Configuration

`up` interacts with a variety of systems, each of which may have information
that should be persisted between commands. `up` stores this information in a
configuration file in `~/.up/config.json`.

## Upbound Configuration

The sections below detail the configuration file format and how to interact with
it.

### Format

`up` allows users to define profiles that contain sets of preferences and
credentials for interacting with Upbound. This enables easily executing commands
as different users, or in different accounts. In the example below, six profiles
are defined: `default`, `dev`, `staging`, `prod`, `ci`, and `selfhosted`.
Commands will use the specified profile when set via the `--profile` flag or
`UP_PROFILE` environment variable. If a profile is not set, `up` will use the
profile specified as `default`, which in this case is actually named `default`.

```json
{
  "upbound": {
    "default": "default",
    "profiles": {
      "default": {
        "id": "hasheddan",
        "type": "user",
        "session": "abcdefg123456789",
        "account": "hasheddan"
      },
      "dev": {
        "id": "hasheddan",
        "type": "user",
        "session": "abcdefg123456789",
        "account": "dev"
      },
      "staging": {
        "id": "hasheddan",
        "type": "user",
        "session": "abcdefg123456789",
        "account": "staging"
      },
      "prod": {
        "id": "hasheddan",
        "type": "user",
        "session": "abcdefg123456789",
        "account": "prod"
      },
      "ci": {
        "id": "faa2d557-9d10-4986-8379-4ad618360e57",
        "type": "token",
        "session": "abcdefg123456789",
        "account": "my-org"
      },
      "selfhosted": {
        "id": "hasheddan",
        "type": "user",
        "session": "abcdefg123456789",
        "account": "myorg",
        "base": {
            "UP_DOMAIN": "https://myorg.com",
            "UP_INSECURE_SKIP_TLS_VERIFY": "true"
        }
      }
}
```

### Specifying Upbound Instance

Because Upbound offers both a hosted and self-hosted product, users may be
logging in and interacting with hosted [Upbound] or their own Upbound
installation. `up` assumes by default that a user is interacting with hosted
Upbound and will use `https://upbound.io` as the domain. However, all commands
that interact with Upbound also accept `--domain` / `UP_DOMAIN`, which overrides
the API endpoint.

### Adding or Updating Profile

To add or update a profile, users can execute `up login` with the appropriate
credentials and a profile name specified. For instance, the following command
would add a new profile named `test`:

```
up login --profile test -u hasheddan -p cool-password
```

If no `--profile` is specified, the profile named `default` will be added or
updated with the relevant credentials. If `-a` (`--account`) is specified it
will be set as the default account for the profile, and if it is not specified
and the credential type is `user`, the username will be used as the account as
well as the ID.

Additionally, `up` offers the `up profile` command group to manage profiles.
Profile-specific environment variables can be set with `up profile config set`
and unset with `up profile config unset`. In the example above, the `selfhosted`
profile will default `UP_DOMAIN` to `https://myorg.com` and
`UP_INSECURE_SKIP_TLS_VERIFY` to `true`.

### Setting the Default Profile

The profile specified as the value to the `default:` key will be used for
commands when `--profile` / `UP_PROFILE` is not set. Any profile can be used as
the default, the the default profile can be changed with `up profile use`. When
a user logs in, the profile used for the login operation will automatically be
set as the `default:`. Additionally, the `up alpha upbound install` command will
automatically set up a profile named `selfhosted` and set it as the default.

### Invalidating Session Tokens

`up` uses session tokens for authentication after login. Tokens are valid for 30
days by default, meaning that a user must login at least once in any 30 day
period. Tokens are sensitive and should not be exposed in any setting. However,
if a token is exposed, assuming it is still present for the given profile in the
configuration file, it can be revoked by running `up logout --profile
<profile-name>`. If a token is exposed and it is no longer present in the
profile and cannot be retrieved for any reason, the user account should reset
its password on [Upbound] immediately. This can be accomplished today by
clicking "Forgot Password?" on the [Upbound login page], following the link in
the email you receive, and making sure to check "Delete all active sessions".

<!-- Named Links -->
[Upbound]: https://www.upbound.io/
[Upbound login page]: https://accounts.upbound.io/login
