const stringify = JSON.stringify;
const stringifyPretty = (object) => stringify(object, null, 2);

const xRequest = new XMLHttpRequest();

const updatePre = (text) => {
    const preCollection = document.getElementsByTagName('pre');
    for (i = 0; i < preCollection.length; ++i) {
        preCollection[i].innerText = text;
    }
};

xRequest.onreadystatechange = function () {
    if (this.readyState == 4 && this.status == 200) {
        const jsonObject = JSON.parse(xRequest.responseText);
        let preText = `Now: ${jsonObject.now}\n\n`;
        preText += `Proxy Duration: ${jsonObject.proxyDuration}\n\n`;
        preText += `GET ${jsonObject.url}\n\n`;
        preText += `Response Status: ${jsonObject.proxyStatus}\n\n`;
        preText += `Response Headers:\n${stringifyPretty(jsonObject.proxyRespHeaders)}\n\n`;
        preText += jsonObject.proxyOutput;
        updatePre(preText);
    }
};

const requestData = (apiPath) => {
    xRequest.open('GET', apiPath, true);
    xRequest.setRequestHeader('Accept', 'application/json');
    xRequest.send();
};

const setTimer = (apiPath) => {
    const checkbox = document.getElementById('autoRefresh');

    setInterval(() => {
        if (checkbox.checked) {
            requestData(apiPath);
        }
    }, 1000);
};

const onload = (requestText, apiPath) => {
    let preText = `Now:\n\n`;
    preText += `Proxy Duration:\n\n`;
    preText += `${requestText}\n\n`;
    preText += 'Response Status:\n\n';
    preText += 'Response Headers:';
    updatePre(preText);

    requestData(apiPath);

    setTimer(apiPath);
};
