function showResult(message, isError = false) {
    if (isError) {
        alert('错误: ' + message);
    } else {
        alert(message);
    }
}

function handleTimeResult(promise) {
    return promise
        .then(() => showResult('设置成功'))
        .catch((error) => {
            console.error('设置失败:', error);
            showResult('设置失败: ' + (error.message || '未知错误'), true);
        });
}

function validateDateInput(input) {
    if (!input.value) {
        showResult('请选择日期', true);
        return false;
    }
    
    const date = new Date(input.value);
    if (isNaN(date.getTime())) {
        showResult('日期格式无效', true);
        return false;
    }
    
    return true;
}

document.getElementById('setTimeBtn').addEventListener('click', () => {
    const timeInput = document.getElementById('timeInput');

    if (!validateDateInput(timeInput)) return;

    const date = new Date(timeInput.value);
    const year = date.getFullYear();
    const month = date.getMonth() + 1;
    const day = date.getDate();
    const timeStr = `${year}/${month}/${day}`;

    console.log('设置日期:', timeStr);
    handleTimeResult(window.electronAPI.setTime(timeStr));
});

document.getElementById('resetTimeBtn').addEventListener('click', () => {
    console.log('恢复时间');
    handleTimeResult(window.electronAPI.setTime('now'));
});
