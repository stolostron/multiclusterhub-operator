'use strict';

const express = require('express');
const path = require('path');

// Constants
const PORT = 3000;
const HOST = '0.0.0.0';

// App
const app = express();

app.get('/charts/:fileName', function(req, res) {
    console.log(req.params)
    var file_name = req.params.fileName
    res.sendFile(path.join(__dirname + '/multiclusterhub/charts/'+file_name));
});

// promises worked locally, but not on cluster

app.get('/readiness', function(req, res) {
    res.sendStatus(200);
});

app.get('/liveness', function(req, res) {
    res.sendStatus(200);
});

app.listen(PORT, HOST);

console.log(`Running on http://${HOST}:${PORT}`);