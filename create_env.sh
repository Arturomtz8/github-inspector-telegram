pack build \
	--builder gcr.io/buildpacks/builder:v1 \
	--env GOOGLE_FUNCTION_SIGNATURE_TYPE=http \
	--env GOOGLE_FUNCTION_TARGET=HandleTelegramWebhook \
    --env GITHUB_BOT_TOKEN=$GITHUB_BOT_TOKEN \
	github-inspector-func