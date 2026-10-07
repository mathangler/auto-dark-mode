Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.
# Offer to launch the app right after a successful install. The checkbox label
# is MUI's own "$(MUI_TEXT_FINISH_RUN)", which the language files localize
# ("运行 Auto Dark Mode" for Simplified Chinese).
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"

# Let the user pick the installer language (English / 简体中文) on startup.
# MUI_LANGDLL_ALWAYSSHOW keeps the dialog on every run (reinstall included) —
# without it MUI silently reuses the remembered language. The choice is still
# written to the registry after each install so the uninstaller runs in the
# last chosen language.
!define MUI_LANGDLL_ALLLANGUAGES # always show every language, no filtering
!define MUI_LANGDLL_ALWAYSSHOW # never skip the dialog on reinstall
!define MUI_LANGDLL_REGISTRY_ROOT "HKCU"
!define MUI_LANGDLL_REGISTRY_KEY "Software\${INFO_COMPANYNAME}"
!define MUI_LANGDLL_REGISTRY_VALUENAME "Installer Language"
!insertmacro MUI_RESERVEFILE_LANGDLL

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # First language = fallback default
!insertmacro MUI_LANGUAGE "SimpChinese" # 简体中文

# Localized strings for the running-process check. The MUI pages localize
# themselves via the language files; these messages are our own.
LangString MsgProcessRunning ${LANG_ENGLISH} "${INFO_PRODUCTNAME} is currently running. Click OK to close it and continue installing, or Cancel to exit the installer."
LangString MsgProcessRunning ${LANG_SIMPCHINESE} "检测到 ${INFO_PRODUCTNAME} 正在运行。点击“确定”将结束该进程并继续安装，点击“取消”将退出安装程序。"
LangString MsgClosingProcess ${LANG_ENGLISH} "Closing the running ${INFO_PRODUCTNAME}..."
LangString MsgClosingProcess ${LANG_SIMPCHINESE} "正在结束运行中的 ${INFO_PRODUCTNAME}..."

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
  !endif
!else
  InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
   !insertmacro MUI_LANGDLL_DISPLAY # ask English / 简体中文 (skipped when silent)
FunctionEnd

# A running instance of the app locks its exe, so the File step below would
# fail. Before copying files, check whether ${PRODUCT_EXECUTABLE} is running
# (tasklist filtered by exact image name, piped through find: exit code 0 =
# found) and offer to close it. Interactive installs ask first; silent
# installs (/S) just close it. Loops until the process is gone or the user
# aborts, in case the kill fails or the app restarts itself.
!macro wails.closeRunningApp
    check_running:
    nsExec::ExecToStack `"$SYSDIR\cmd.exe" /C tasklist /NH /FO CSV /FI "IMAGENAME eq ${PRODUCT_EXECUTABLE}" | find /I "${PRODUCT_EXECUTABLE}"`
    Pop $0 # find exit code: 0 = process is running
    Pop $1 # command output (unused, but popped to keep the stack clean)
    IntCmp $0 0 process_running done done
    process_running:
    IfSilent close_running prompt_user
    prompt_user:
    MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "$(MsgProcessRunning)" IDOK close_running
    Abort # user chose Cancel: leave the installer
    close_running:
    DetailPrint "$(MsgClosingProcess)"
    nsExec::Exec `"$SYSDIR\taskkill.exe" /F /T /IM "${PRODUCT_EXECUTABLE}"`
    Pop $0 # taskkill exit code (ignored: the loop re-checks instead)
    Sleep 500 # give the OS a moment to release the file handles
    Goto check_running
    done:
!macroend

Section
    !insertmacro wails.setShellContext

    # Must happen before wails.files: the exe can't be replaced while running.
    !insertmacro wails.closeRunningApp

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    # No prompt here — the user already decided to uninstall. Close the app
    # and wait a moment so its files can be removed.
    nsExec::Exec `"$SYSDIR\taskkill.exe" /F /T /IM "${PRODUCT_EXECUTABLE}"`
    Pop $0
    Sleep 500

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller

    DeleteRegKey HKCU "Software\${INFO_COMPANYNAME}" # installer language choice
SectionEnd
