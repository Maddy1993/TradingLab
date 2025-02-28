FROM node:16-alpine as build

WORKDIR /app

# Copy package files and install dependencies
COPY ui/package.json ui/package-lock.json* ./

# Add the lightweight-charts package explicitly
RUN npm install && npm install --save lightweight-charts@4.1.1

# Copy source code
COPY ui/public ./public
COPY ui/src ./src

# Build the app
RUN npm run build

# Production stage
FROM nginx:alpine

# Copy the build output
COPY --from=build /app/build /usr/share/nginx/html

# Copy nginx config
COPY ui/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]