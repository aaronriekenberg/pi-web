const updatePre = (text) => {
    const preCollection = document.getElementsByTagName('pre');
    for (i = 0; i < preCollection.length; ++i) {
        preCollection[i].innerText = text;
    }
};

const handleFetchResponse = (jsonObject) => {
    let command = jsonObject.command;
    for (const arg of (jsonObject.args || [])) {
        command += ` ${arg}`;
    }
    let preText = `Now: ${jsonObject.now}\n\n`;
    preText += `Command Duration: ${jsonObject.commandDuration}\n\n`;
    preText += `$ ${command}\n\n`;
    preText += jsonObject.commandOutput;
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

const onload = (commandText, apiPath) => {
    let preText = `Now:\n\n`;
    preText += `Command Duration:\n\n`;
    preText += `$ ${commandText}`;
    updatePre(preText);

    fetchData(apiPath);

    setTimer(apiPath);
};
