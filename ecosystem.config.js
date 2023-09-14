const os = require('os');
module.exports = {
    apps: [{
        port        : 30812,
        name        : "linka.metric",
        script      : "server/dist", // 👈 CommonJS
        watch       : false,           
        instances   : 1,
        exec_mode   : 'fork',         
    }]
}
