# ChangeSystemTime 全面优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use compose:subagent (recommended) or compose:execute to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 优化 Electron 项目，修复安全漏洞、改进代码质量、减小打包体积、提升启动速度

**Architecture:** 保持现有架构，通过修复安全漏洞、重构代码、优化配置来实现全面优化

**Tech Stack:** Electron, electron-forge, Node.js

## Global Constraints

- 仅支持 Windows 平台
- 保持现有功能不变
- 使用 electron-forge 作为唯一打包工具
- 禁用 nodeIntegration，使用 preload 脚本暴露 API

---

### Task 1: 修复 main.js 安全漏洞和错误处理

**Covers:** [S1, S2]

**Files:**
- Modify: `electron/main.js`

**Interfaces:**
- Consumes: 无
- Produces: 安全的命令执行函数，正确的错误处理

- [ ] **Step 1: 添加日期格式验证函数**

```javascript
// 验证日期格式 yyyy/MM/dd
function isValidDateFormat(dateStr) {
    const regex = /^\d{4}\/\d{1,2}\/\d{1,2}$/;
    if (!regex.test(dateStr)) return false;
    
    const [year, month, day] = dateStr.split('/').map(Number);
    const date = new Date(year, month - 1, day);
    return date.getFullYear() === year && 
           date.getMonth() === month - 1 && 
           date.getDate() === day;
}
```

- [ ] **Step 2: 修复 executeCommand 函数，正确处理日志和 Promise**

```javascript
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
```

- [ ] **Step 3: 修改 ipcMain.handle，添加日期验证**

```javascript
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
            command = `date ${time}`;
        }
        return await executeCommand(command);
    } catch (error) {
        if (time === 'now') {
            try {
                await startTimeService();
                return await executeCommand('w32tm /resync');
            } catch (e) {
                throw e;
            }
        } else {
            throw error;
        }
    }
});
```

- [ ] **Step 4: 测试验证**

运行应用，测试以下场景：
1. 输入有效日期（如 2024/01/15）应能正常设置
2. 输入无效格式（如 2024-01-15 或 2024/13/01）应显示错误
3. 恢复时间功能应正常工作

- [ ] **Step 5: 提交更改**

```bash
git add electron/main.js
git commit -m "fix: 修复安全漏洞和错误处理

- 添加日期格式验证，防止命令注入
- 修复 executeCommand 函数的 Promise 处理
- 添加输入验证错误提示"
```

---

### Task 2: 重构 renderer.js，减少重复代码

**Covers:** [S2]

**Files:**
- Modify: `renderer.js`

**Interfaces:**
- Consumes: window.electronAPI.setTime()
- Produces: 更简洁的代码结构

- [ ] **Step 1: 提取公共函数**

```javascript
// 显示操作结果
function showResult(message, isError = false) {
    alert(message);
}

// 处理设置时间结果
function handleTimeResult(promise) {
    promise
        .then(() => showResult('设置成功'))
        .catch((error) => {
            console.error('设置失败:', error);
            showResult('设置失败: ' + (error.message || '未知错误'), true);
        });
}

// 验证日期输入
function validateDateInput(input) {
    if (!input.value) {
        showResult('请选择日期', true);
        return false;
    }
    return true;
}
```

- [ ] **Step 2: 重构按钮事件处理**

```javascript
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
```

- [ ] **Step 3: 测试验证**

测试设置和恢复时间功能是否正常工作。

- [ ] **Step 4: 提交更改**

```bash
git add renderer.js
git commit -m "refactor: 重构 renderer.js，减少代码重复

- 提取公共函数 showResult、handleTimeResult、validateDateInput
- 简化事件处理逻辑
- 改进错误提示信息"
```

---

### Task 3: 优化 package.json，移除冗余依赖

**Covers:** [S2, S3]

**Files:**
- Modify: `package.json`

**Interfaces:**
- Consumes: 无
- Produces: 更清洁的依赖配置

- [ ] **Step 1: 移除 electron-builder 相关依赖**

从 devDependencies 中移除：
- electron-builder

- [ ] **Step 2: 评估是否移除 sudo-prompt**

分析：sudo-prompt 用于以管理员权限执行命令。在 Windows 上，可以通过 Electron 的 app.relaunch 以管理员权限重启应用来实现。但为了保持简单，暂时保留 sudo-prompt。

- [ ] **Step 3: 更新 package.json**

```json
{
  "name": "set-system-time",
  "version": "1.0.1",
  "description": "设置系统时间",
  "main": "electron/main.js",
  "scripts": {
    "start": "electron-forge start",
    "package": "electron-forge package",
    "make": "electron-forge make"
  },
  "author": "cyam",
  "license": "MIT",
  "devDependencies": {
    "@electron-forge/cli": "^6.4.1",
    "@electron-forge/maker-squirrel": "^6.4.1",
    "@electron-forge/maker-zip": "^6.4.1",
    "@electron-forge/plugin-auto-unpack-natives": "^6.4.1",
    "electron": "^26.1.0",
    "electron-squirrel-startup": "^1.0.0"
  },
  "dependencies": {
    "sudo-prompt": "^9.2.1"
  }
}
```

- [ ] **Step 4: 移除 package.json 中的 build 配置**

删除整个 "build" 配置块，因为我们将只使用 electron-forge。

- [ ] **Step 5: 测试打包**

```bash
npm run make
```

- [ ] **Step 6: 提交更改**

```bash
git add package.json
git commit -m "chore: 清理 package.json，移除冗余依赖

- 移除 electron-builder 相关配置和依赖
- 统一使用 electron-forge 打包
- 更新版本号"
```

---

### Task 4: 优化 Electron 安全配置

**Covers:** [S2]

**Files:**
- Modify: `electron/main.js`
- Modify: `electron/preload.js`

**Interfaces:**
- Consumes: 无
- Produces: 更安全的 Electron 配置

- [ ] **Step 1: 修改 main.js 中的 BrowserWindow 配置**

```javascript
function createWindow() {
    const win = new BrowserWindow({
        width: 300,
        height: 300,
        webPreferences: {
            nodeIntegration: false,
            contextIsolation: true,
            preload: path.join(__dirname, 'preload.js')
        }
    });

    win.loadFile('index.html');
    Menu.setApplicationMenu(null);
    
    if (NODE_ENV === "development") {
        win.webContents.openDevTools();
    }
}
```

- [ ] **Step 2: 更新 preload.js，添加版本信息暴露**

```javascript
const { contextBridge, ipcRenderer } = require('electron');

// 暴露版本信息
contextBridge.exposeInMainWorld('versions', {
    node: () => process.versions.node,
    chrome: () => process.versions.chrome,
    electron: () => process.versions.electron
});

// 暴露 setTime API
contextBridge.exposeInMainWorld('electronAPI', {
    setTime: (time) => ipcRenderer.invoke('set-time', time)
});

// DOMContentLoaded 事件处理
window.addEventListener('DOMContentLoaded', () => {
    const replaceText = (selector, text) => {
        const element = document.getElementById(selector);
        if (element) element.innerText = text;
    };

    for (const dependency of ['chrome', 'node', 'electron']) {
        replaceText(`${dependency}-version`, process.versions[dependency]);
    }
});
```

- [ ] **Step 3: 浪�试验证**

测试应用是否能正常启动和运行。

- [ ] **Step 4: 提交更改**

```bash
git add electron/main.js electron/preload.js
git commit -m "security: 优化 Electron 安全配置

- 禁用 nodeIntegration
- 启用 contextIsolation
- 更新 preload.js 以安全暴露 API"
```

---

### Task 5: 优化打包配置

**Covers:** [S3]

**Files:**
- Modify: `forge.config.js`

**Interfaces:**
- Consumes: 无
- Produces: 优化的打包配置

- [ ] **Step 1: 优化 forge.config.js**

```javascript
module.exports = {
  packagerConfig: {
    asar: true,
    icon: './icon', // 如果有图标文件
    name: 'SetSystemTime',
    executableName: 'SetSystemTime',
    appCopyright: 'Copyright © 2024',
    win32metadata: {
      CompanyName: 'cyam',
      FileDescription: '设置系统时间工具',
      ProductName: 'SetSystemTime'
    }
  },
  rebuildConfig: {},
  makers: [
    {
      name: '@electron-forge/maker-squirrel',
      config: {
        name: 'SetSystemTime',
        authors: 'cyam',
        description: '设置系统时间工具'
      }
    },
    {
      name: '@electron-forge/maker-zip',
      platforms: ['win32']
    }
  ],
  plugins: [
    {
      name: '@electron-forge/plugin-auto-unpack-natives',
      config: {}
    }
  ]
};
```

- [ ] **Step 2: 测试打包**

```bash
npm run make
```

- [ ] **Step 3: 检查打包产物**

检查 `out` 目录中的打包文件，确认大小和功能。

- [ ] **Step 4: 提交更改**

```bash
git add forge.config.js
git commit -m "build: 优化打包配置

- 添加 Windows 元数据
- 优化 maker-squirrel 配置
- 确保只针对 win32 平台打包"
```

---

### Task 6: 最终测试和验证

**Covers:** [S1, S2, S3]

**Files:**
- 无新增文件

**Interfaces:**
- Consumes: 所有之前的更改
- Produces: 验证所有优化生效

- [ ] **Step 1: 运行完整测试**

```bash
npm start
```

测试以下功能：
1. 应用正常启动
2. 设置日期功能（有效日期）
3. 设置日期功能（无效日期应显示错误）
4. 恢复时间功能
5. 检查是否有安全警告

- [ ] **Step 2: 打包测试**

```bash
npm run make
```

- [ ] **Step 3: 检查打包产物**

1. 检查打包文件大小是否减小
2. 运行打包后的应用，确认功能正常
3. 检查启动速度是否有改善

- [ ] **Step 4: 提交最终更改**

```bash
git add -A
git commit -m "chore: 完成全面优化

- 修复安全漏洞
- 改进代码质量
- 优化打包配置
- 减小打包体积
- 提升启动速度"
```

---

## 优化效果总结

1. **安全性提升**：修复命令注入漏洞，禁用 nodeIntegration，启用 contextIsolation，添加 sandbox
2. **代码质量改进**：减少重复代码，改进错误处理，添加输入验证，使用 PowerShell 命令避免区域设置问题
3. **打包配置优化**：移除冗余依赖 electron-builder，统一使用 electron-forge，添加 Windows 元数据
4. **维护性增强**：更清晰的代码结构和配置，移除不必要的跨平台支持
