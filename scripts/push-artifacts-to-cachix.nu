#!/usr/bin/env nu

let is_pull_request = $env.GITHUB_EVENT_NAME? == "pull_request"
let ability_cachix_auth_token = $env.CACHIX_AUTH_TOKEN? | is-not-empty

# early return via $ability_cachix_auth_token & $is_pull_request
if (not $ability_cachix_auth_token and not $is_pull_request) {
  error make { msg: "There are no setting for CACHIX_AUTH_TOKEN environment variable." }
}

# Push to Nix store on cachix, from piped string value
def push-or [ability_cachix_auth_token: bool, is_pull_request: bool] {
  if ($ability_cachix_auth_token and not $is_pull_request) {
    $in | cachix push haruki7049
  } else {
    # Drain the input so the producer (e.g. `nix build --print-out-paths`) does not hit a broken pipe
    let paths = $in | collect | str trim
    print $"Skipping 'cachix push haruki7049' for:\n($paths)"
  }
}

print "Pushing flake inputs..."
print

nix flake archive --json
  | from json
  | [ ($in | get path), ($in | get inputs | values | get path) ]
  | flatten
  | str join (char nl)
  | push-or $ability_cachix_auth_token $is_pull_request

print "Pushing runtime closure..."
print

nix build --no-link --print-out-paths '.#spelling'
  | push-or $ability_cachix_auth_token $is_pull_request
