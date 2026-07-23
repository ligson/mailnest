ARG NODE_BASE=node:22-alpine
ARG NGINX_BASE=nginx:1.27-alpine
FROM ${NODE_BASE} AS build

WORKDIR /src

COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM ${NGINX_BASE}

COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 80
