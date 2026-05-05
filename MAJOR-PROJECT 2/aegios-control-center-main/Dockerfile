FROM node:22-alpine

WORKDIR /app

# Copy package files
COPY package.json package-lock.json* bun.lockb* ./

# Install all dependencies required for `npm run dev` (including devDependencies like vite)
RUN npm ci --include=dev

# Copy source code
COPY . .

# Expose the port (Vite will be configured to listen on 8081)
EXPOSE 8081

# Command to run in development mode
# We use --host :: to ensure it listens on all interfaces (for Docker connectivity)
# We override the port to 8081
CMD ["npm", "run", "dev", "--", "--host", "::", "--port", "8081"]
