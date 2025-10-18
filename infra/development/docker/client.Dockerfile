FROM node:20-alpine

WORKDIR /app

# Install dependencies
COPY package*.json ./
RUN npm install

# Expose Vite dev server port
EXPOSE 5173

# Start Vite dev server with host flag to allow external connections
CMD ["sh", "-c", "npm run dev -- --host 0.0.0.0"]

