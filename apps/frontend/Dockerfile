# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

COPY apps/frontend/package*.json ./
RUN npm ci

COPY apps/frontend/ ./
RUN npm run build

# Production stage
FROM node:22-alpine

# Set to production mode
ENV NODE_ENV=production

WORKDIR /app

COPY --from=builder /app/package*.json ./
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/node_modules ./node_modules

# Ensure correct file permissions
RUN chown -R node:node /app

# Run as non-privileged node user
USER node

EXPOSE 3000

STOPSIGNAL SIGTERM

CMD ["npm", "start"]
