# whistleslacker
#### Make a private slack channel public. Or, at least, moving everyone over to a public channel...

It's recommended to use your token used on the admin page (run `boot_data.api_token` inside the console on the admin page) as it would otherwise block you from switching a user from single-channel (ultra restricted) to a multi-channel (restricted) guest.

# Example

```
go run main.go -token "xoxs-hurr-durr" -revert-to-single-channel-guest channel-1 channel-2
```

This will use token 'xoxs-hurr-durr' to (re)create `channel-1` and `channel-2`, and rename the old `channel-1` to `channel-1-old`.
