ARG NODE_VERSION=22
FROM node:${NODE_VERSION}-slim

ARG CLAUDE_VERSION=2.1.210

RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates ripgrep \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_VERSION}

COPY entrypoint.sh /usr/local/bin/benchy-entrypoint
RUN chmod +x /usr/local/bin/benchy-entrypoint

USER node
ENV HOME=/home/node
ENV CLAUDE_CONFIG_DIR=/home/node/.claude
WORKDIR /work

ENTRYPOINT ["/usr/local/bin/benchy-entrypoint"]
