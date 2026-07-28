module.exports = {
  packagerConfig: {
    asar: true,
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
