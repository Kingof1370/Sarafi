# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

COPY apps/frontend/package*.json ./
RUN npm install

COPY apps/frontend/ ./
RUN npm run build

# Production stage
FROM node:22-alpine

WORKDIR /app

COPY --from=builder /app/package*.json ./
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/node_modules ./node_modules

EXPOSE 3000

CMD ["npm", "start"]
