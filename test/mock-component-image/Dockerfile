# Copyright Contributors to the Open Cluster Management project

FROM node:14

WORKDIR /usr/src/app

COPY package*.json ./

RUN npm install

RUN apt-get update && \
    apt-get install -y openssl && \
    openssl req -new -newkey rsa:4096 -nodes -keyout localhost.key -out localhost.csr -subj "/C=NA/ST=NA/L=NA/O=NA/CN=NA" && \
    openssl  x509  -req  -days 365  -in localhost.csr  -signkey localhost.key  -out localhost.crt

RUN chmod 777 localhost.key 

COPY . .

COPY ./bin/kubectl /
COPY ./bin/controller /
COPY ./bin/proxyserver /
COPY ./bin/webhook /

EXPOSE 3000
CMD [ "node", "app.js" ]