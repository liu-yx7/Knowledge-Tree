# Stage 1: Build the React app
FROM node:20-alpine AS builder

# Set the working directory
WORKDIR /build

# Copy package.json and package-lock.json
COPY package*.json pnpm-lock.yaml /build

# Install pnpm
# RUN npm install -g pnpm

# Install dependencies
RUN npm install 

# Copy the rest of the application code
COPY . /build

# Build the React app
RUN npm run build

# Stage 2: Serve the app with Nginx
FROM nginx:alpine

# Copy the built React app from the build stage
COPY --from=builder /build/dist /usr/share/nginx/html
# cp -r dist/* /usr/share/nginx/html


# Copy the custom Nginx configuration file
COPY nginx.conf /etc/nginx/conf.d/default.conf
# scp nginx.conf /etc/nginx/conf.d/default.conf

# Expose port 80
EXPOSE 80

# Start Nginx
CMD ["nginx", "-g", "daemon off;"]