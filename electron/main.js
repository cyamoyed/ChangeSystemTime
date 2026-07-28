// main.js
const {
    app,
    BrowserWindow,
    Menu,
    ipcMain
} = require('electron');
const path = require('path');
const sudo = require('sudo-prompt');

const NODE_ENV = process.env.NODE_ENV;

// 验证日期格式 yyyy/MM/dd
function isValidDateFormat(dateStr) {
    const regex = /^\d{4}\/\d{1,2}\/\d{1,2}$/;
    if (!regex.test(dateStr)) return false;
    
    const [year, month, day] = dateStr.split('/').map(Number);

    // 范围校验
    if (year < 1900 || year > 2100) return false;
    if (month < 1 || month > 12) return false;
    if (day < 1 || day > 31) return false;

    const date = new Date(year, month - 1, day);
    return date.getFullYear() === year &&
           date.getMonth() === month - 1 &&
           date.getDate() === day;
}

function createWindow() {
    const win = new BrowserWindow({
        width: 300,
        height: 300,
        webPreferences: {
            nodeIntegration: false,
            contextIsolation: true,
            sandbox: true,
            preload: path.join(__dirname, 'preload.js')
        }
    });

    win.loadFile('index.html');
    // 关闭菜单
    Menu.setApplicationMenu(null);
    // 打开开发工具
    if (NODE_ENV === "development") {
        win.webContents.openDevTools();
    }
}

function executeCommand(command) {
    return new Promise((resolve, reject) => {
        sudo.exec(command, (error, stdout, stderr) => {
            if (error) {
                console.error(`exec error: ${error}`);
                reject(error);
                return;
            }
            console.log(`stdout: ${stdout}`);
            if (stderr) {
                console.error(`stderr: ${stderr}`);
            }
            resolve(stdout);
        });
    });
}

function startTimeService() {
    const command = 'net start w32time';
    return executeCommand(command);
}

app.whenReady().then(() => {
    ipcMain.handle('set-time', async (event, time) => {
        try {
            let command;
            if (time === 'now') {
                command = 'w32tm /resync';
            } else {
                // 验证日期格式
                if (!isValidDateFormat(time)) {
                    throw new Error('日期格式无效，请使用 yyyy/MM/dd 格式');
                }
                const [year, month, day] = time.split('/');
                command = `powershell -Command "Set-Date -Date '${year}-${month}-${day}'"`;
            }
            return await executeCommand(command);
        } catch (error) {
            if (time === 'now') {
                await startTimeService();
                return await executeCommand('w32tm /resync');
            } else {
                throw error;
            }
        }
    });

    createWindow();

    app.on('activate', function () {
        if (BrowserWindow.getAllWindows().length === 0) createWindow();
    });
});

app.on('window-all-closed', function () {
    app.quit();
});
