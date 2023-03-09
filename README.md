# enhanced_github_presense
Like discord presence but github and with a typo.

## Setup

1. Update `appsStatus.json` to your preferred settings.
2. Add Github PAT with _user_ permissions to environment variable `GITHUB_PAT`.
3. Start the application.
4. (optional) If you're very weird you could run this as a system service I guess. Please let me know if you do, for science.

## gql libraries (and my sleep-deprived brain)

Autogenerated and type-safe gql interfaces are very cool but sometimes lead to issues if you (like me) don't really know what you're doing.

Spot the issue with the request generated by the library vs the one generated by the github api explorer:

`{"query":"mutation ($input:ChangeUserStatusInput!){changeUserStatus(input: $input){clientMutationId,status{message}}}","variables":{"input":{"clientMutationId":"ABC","emoji":"","expiresAt":"0001-01-01T00:00:00Z","limitedAvailability":false,"message":"test","organizationId":""}}}`

`{"query":"mutation ($input: ChangeUserStatusInput!) {\n  changeUserStatus(input: $input) {clientMutationId,status {\n      message\n    }\n  }\n}\n","variables":{"input":{"clientMutationId":"69","message":""}}}` 

The issue is that in the first (lib generated) version, certain fields are populated with empty values. Github's service rejects this because it's looking for an org that doesn't exist. Then you get this lovely response from the server: `"message": "Could not resolve to a node with the global id of ''"`. Really helpful. 

[https://github.com/Khan/genqlient/](https://github.com/Khan/genqlient/) is quite cool but stole many hours of my life.
