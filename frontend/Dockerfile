FROM node:22-alpine AS builder

WORKDIR /build
COPY package.json ./
RUN npm install --legacy-peer-deps

COPY . .
RUN npm run build

FROM node:22-alpine AS runner
RUN npm install -g serve
COPY --from=builder /build/dist /app
EXPOSE 80
CMD ["serve", "-s", "/app", "-l", "80"]
