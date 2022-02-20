const stringify = JSON.stringify;
const stringifyPretty = (object) => stringify(object, null, 2);

const updatePre = (text) => {
    const preCollection = document.getElementsByTagName('pre');
    for (i = 0; i < preCollection.length; ++i) {
        preCollection[i].innerText = text;
    }
};

const handleFetchResponse = (jsonObject) => {
    let preText = `Now: ${jsonObject.now}\n\n`;
    preText += `Proxy Duration: ${jsonObject.proxyDuration}\n\n`;
    preText += `GET ${jsonObject.proxyInfo.url}\n\n`;
    preText += `Response Status: ${jsonObject.proxyStatus}\n\n`;
    preText += `Response Headers:\n${stringifyPretty(jsonObject.proxyRespHeaders)}\n\n`;
    preText += jsonObject.proxyOutput;
    updatePre(preText);
}

const fetchData = async (apiPath) => {
    try {
        const response = await fetch(apiPath, {
            method: 'GET',
            headers: {
                'Accept': 'application/json'
            }
        });
        const jsonObject = await response.json();
        handleFetchResponse(jsonObject);
    } catch (error) {
        console.error('fetch error:', error);
    }
};

const setTimer = (apiPath) => {
    const checkbox = document.getElementById('autoRefresh');

    setInterval(() => {
        if (checkbox.checked) {
            fetchData(apiPath);
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

    fetchData(apiPath);

    setTimer(apiPath);
};
