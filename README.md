# Hyperliqid Validator Monitoring

A lightweight Go application for monitoring Hyperliqid blockchain validators and sending Discord notifications about their status changes. This tool helps node operators stay informed about critical validator state changes (jailed and active status) without constant manual monitoring.

![validator](hl-validator-jailed-notification-discord.png)

## Features

- **Status Monitoring**: Tracks validator jailed status and activity in real-time
- **Recovery Notifications**: Sends recovery notifications when validators return to normal
- **Notification Backoff**: Implements exponential backoff to prevent notification spam
- **Containerized**: Designed to run in container for easy deployment
- **Low Resource Usage**: Minimal CPU and memory footprint

## Requirements

- Docker (for containerized development/testing)
- Discord webhook URL for notifications
- golang (for local development/testing)

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `API_ENDPOINT` | Hyperliquid API endpoint for fetching validator data </br> testnet: `https://api.hyperliquid-testnet.xyz/info` </br> mainnet: `https://api.hyperliquid.xyz/info` | Required |
| `VALIDATOR_ADDRESS` | On-chain address of the validator to monitor | Required |
| `DISCORD_WEBHOOK` | Discord webhook URL for notifications | Required |
| `CRON_INTERVAL` | Monitoring check interval (e.g., "1m", "5m") | "1m" |

## Quick Start

1. Create a Discord webhook in your server settings
2. Find your validator's address (e.g., `0x39b9cb9e48ec50b7b8fcdfadeabec2642c84b9ae`)
3. Pull and run the Docker image:

```bash
docker run -d --name go-hl-val-mon \
  -e API_ENDPOINT="https://api.hyperliquid.xyz/info" \
  -e VALIDATOR_ADDRESS="0x39b9cb9e48ec50b7b8fcdfadeabec2642c84b9ae" \
  -e DISCORD_WEBHOOK="https://discord.com/api/webhooks/your-webhook-url" \
  reg.nodeops.xyz/public/hl-val-moni
```

## Building From Source

```bash
git clone https://github.com/NodeOps-app/Hyperliquid-validator-monitoring
cd go-hl-val-mon
```

## Run locally

```bash
export API_ENDPOINT="https://api.hyperliquid.xyz/info"
export VALIDATOR_ADDRESS="0x39b9cb9e48ec50b7b8fcdfadeabec2642c84b9ae"
export DISCORD_WEBHOOK="https://discord.com/api/webhooks/your-webhook-url"
export CRON_INTERVAL="1m"
go run main.go
```

## Docker Build

```bash
go build -trimpath -ldflags="-s -w -X 'runtime/internal/sys.DefaultGoroot=unknown' -X 'runtime/internal/sys.DefaultCompiler=unknown' -extldflags '-Wl,-z,relro,-z,now,-z,noexecstack,-fPIC'" -o go-hl-val-mon ./...
docker build -t go-hl-val-mon .
docker run -d --name go-hl-val-mon \
  -e API_ENDPOINT="https://api.hyperliquid.xyz/info" \
  -e VALIDATOR_ADDRESS="0x39b9cb9e48ec50b7b8fcdfadeabec2642c84b9ae" \
  -e DISCORD_WEBHOOK="https://discord.com/api/webhooks/your-webhook-url" \
  -e CRON_INTERVAL="1m" \
  go-hl-val-mon
```

## Finding Your Validator Address

You can find your validator address using the Hyperliquid API:

```shell
curl -sLX POST --header "Content-Type: application/json" --data '{ "type": "validatorSummaries"}' https://api.hyperliquid.xyz/info | jq '.[] | select(.name == "YourValidatorName")'
```

Look for the `validator` field in the output, which contains the address.

## Notification Types

- 🚨 **Alert**: Validator is jailed or inactive
- ✅ **Recovery**: Validator has recovered from jailed state or is active again

## Use Cases

- **Validator Operators**: Monitor your own validators to quickly respond to issues
- **Delegators**: Keep track of validators where you've staked tokens
- **Network Observers**: Track the health of major validators in a network

## Upcoming features

- **Block signing monitoring**: Monitor if the validator is signing blocks alongwith additional performance metrics (e.g., block signing rate, uptime)
- **Performance Metrics**:
- **Multiple Validator Monitoring**: Support for monitoring multiple validators simultaneously
- **Customizable Notification Channels**: Support for other notification platforms (e.g., Slack, email)
- **Alerting Thresholds**: Set custom thresholds for alerts based on validator performance metrics
- **Web UI**: A simple web interface for monitoring and configuring the tool
- **Integration with Monitoring Tools**: Integrate with existing monitoring solutions (e.g., Prometheus, Grafana)
- ***Testing**: Unit tests and integration tests for better code quality*
- **Configuration File**: Support for a configuration file to manage settings instead of environment variables
- **Rate Limiting**: Implement rate limiting for API requests to avoid hitting the API too frequently
- **Customizable Notification Messages**: Allow users to customize the content of notifications sent to Discord

## Extending the Tool

This tool can be extended to monitor additional validator metrics or integrate with other notification systems. Pull requests welcome!

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
