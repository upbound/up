# Configuration

`up` interacts with a variety of systems and stores persistent information in a
configuration file in `~/.up/config.json`.

## Upbound configuration

The configuration file stores information for interacting with Upbound. The sections below detail the format and how to interact with it. 
### Format

`up` allows users to define profiles that contain sets of preferences and
credentials for interacting with Upbound. Enabling execution of commands
as different users, or in different accounts. The example below defines five profiles: `default`, `dev`, `staging`, `prod`, and `ci`. Commands
use the specified profile when set via the `--profile` flag or `UP_PROFILE`
environment variable. If a profile isn't set, `up` uses the profile
specified as `default`, which in this case is actually named `default`.

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
}
```

### Specifying the Upbound instance

Upbound offers both a hosted and self-hosted product. Users log in and interact with [Upbound Cloud] or their own [Upbound
Enterprise] installation. By default `up` uses `https://api.upbound.io` as the host endpoint.
All commands that interact with Upbound also accept an `--endpoint` /
`UP_ENDPOINT`, which overrides the API endpoint.

### Adding or updating profile

To add or update a profile, run `up login` with the appropriate
credentials and a profile name specified. For instance, the following command
would add a new profile named `test`:

```console
up login --profile test -u hasheddan -p cool-password
```

Use `--profile` to specify a specific profile. If a profile isn't provided `up login` creates or updates the `default` profile. 
Specify `-a` (`--account`) to set the default account for the profile. The default credential type is `user`. The default `account` and `id` is the username.

### Setting the default profile

Modifying the default profile with the `up` command line isn't supported today.

To change the `default` profile edit `~/.up/config.json` and define the `default:` key.

### Invalidating session tokens

`up` uses session tokens for authentication after log in. Tokens are valid for 30
days by default., A user must log in at least once every 30 days. 

To revoke a token still present for the given profile in the
configuration file, use the command `up logout --profile <profile-name>`. To revoke a token no longer present in the
profile reset the user password on [Upbound Cloud]. Select "Forgot Password?" on the [Upbound Cloud login page], following the
link in the email you receive, and select "Delete all active
sessions" option.

<!-- Named Links -->
[Upbound Cloud]: ../../upbound-cloud
[Upbound Enterprise]: ../../upbound-enterprise
[Upbound Cloud login page]: https://cloud.upbound.io/login