using System.Runtime.InteropServices;
using System.Text.Json;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.WinForms;

namespace PermissionProtector.Desktop;

internal sealed class MainForm : Form
{
    private const int WM_NCLBUTTONDOWN = 0xA1;
    private const int HTCAPTION = 0x2;

    private readonly DesktopRuntime runtime = new();
    private readonly WebView2 webView = new();
    private readonly StartupExperienceControl startupExperience;
    private Icon? ownedWindowIcon;
    private WebResizeSession? activeWebResize;

    public MainForm()
    {
        Text = "OpenAD";
        ownedWindowIcon = DesktopBranding.CreateApplicationIcon(Application.ExecutablePath);
        Icon = ownedWindowIcon;
        StartPosition = FormStartPosition.CenterScreen;
        FormBorderStyle = FormBorderStyle.None;
        MinimumSize = new Size(980, 680);
        Size = new Size(1280, 860);
        BackColor = Color.FromArgb(14, 15, 19);

        webView.Dock = DockStyle.Fill;
        webView.Visible = false;
        Controls.Add(webView);
        startupExperience = new StartupExperienceControl(
            StartupLocaleStore.Resolve(runtime.DataDirectory));
        startupExperience.RetryRequested += async (_, _) => await BootAsync();
        startupExperience.MinimizeRequested += (_, _) => WindowState = FormWindowState.Minimized;
        startupExperience.CloseRequested += (_, _) => Close();
        startupExperience.DragRequested += (_, _) => BeginWindowDrag();
        Controls.Add(startupExperience);
        startupExperience.BringToFront();

        Shown += async (_, _) =>
        {
            startupExperience.BeginStartup();
            startupExperience.BringToFront();
            startupExperience.Refresh();
            await Task.Delay(40);
            await BootAsync();
        };
        FormClosing += (_, _) => runtime.Dispose();
        FormClosed += (_, _) => ownedWindowIcon?.Dispose();
    }

    [DllImport("user32.dll")]
    private static extern bool ReleaseCapture();

    [DllImport("user32.dll")]
    private static extern IntPtr SendMessage(IntPtr hWnd, int message, int wParam, int lParam);

    protected override CreateParams CreateParams
    {
        get
        {
            var parameters = base.CreateParams;
            parameters.Style = NativeWindowChrome.EnableBorderlessResizing(parameters.Style);
            return parameters;
        }
    }

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        UpdateMaximizedBounds();
    }

    protected override void OnLocationChanged(EventArgs e)
    {
        base.OnLocationChanged(e);
        if (WindowState != FormWindowState.Maximized)
        {
            UpdateMaximizedBounds();
        }
    }

    protected override void WndProc(ref Message m)
    {
        if (SingleInstanceActivation.IsActivationMessage(m.Msg))
        {
            RestoreAndActivate();
            m.Result = IntPtr.Zero;
            return;
        }

        if (NativeWindowChrome.TrySuppressNonClientFrame(ref m))
        {
            return;
        }

        if (m.Msg == NativeWindowChrome.WmNcHitTest)
        {
            m.Result = WindowState == FormWindowState.Maximized
                ? (IntPtr)NativeWindowChrome.HtClient
                : (IntPtr)NativeWindowChrome.HitTestResizeBorder(
                    PointToClient(NativeWindowChrome.ScreenPointFromLParam(m.LParam)),
                    ClientSize,
                    DeviceDpi);
            return;
        }

        base.WndProc(ref m);
    }

    private void RestoreAndActivate()
    {
        if (WindowState == FormWindowState.Minimized)
        {
            WindowState = FormWindowState.Normal;
        }

        Show();
        BringToFront();
        Activate();
        SingleInstanceActivation.RestoreAndActivateWindow(Handle);
    }

    private async Task BootAsync()
    {
        webView.Visible = false;
        startupExperience.BeginStartup();
        startupExperience.Visible = true;
        startupExperience.BringToFront();

        try
        {
            await runtime.StartAsync(SetStatus, CancellationToken.None);
            await InitializeWebViewAsync();
            webView.Source = runtime.AppUrl;
        }
        catch (Exception ex)
        {
            SetStatus("Startup failed: " + ex.Message);
        }
    }

    private async Task InitializeWebViewAsync()
    {
        if (webView.CoreWebView2 is not null)
        {
            return;
        }

        SetStatus("Starting desktop window...");
        var environment = await CoreWebView2Environment.CreateAsync(userDataFolder: runtime.WebViewDataDirectory);
        await webView.EnsureCoreWebView2Async(environment);
        var core = webView.CoreWebView2 ?? throw new InvalidOperationException("WebView2 initialization failed.");
        core.Settings.AreDefaultContextMenusEnabled = true;
        core.Settings.AreDevToolsEnabled = true;
        core.WebMessageReceived += HandleWebMessageReceived;
        webView.NavigationCompleted += async (_, e) =>
        {
            if (e.IsSuccess)
            {
                if (!await startupExperience.CompleteAsync())
                {
                    return;
                }
                startupExperience.Visible = false;
                webView.Visible = true;
                webView.Focus();
            }
            else
            {
                SetStatus("Page load failed: " + e.WebErrorStatus);
            }
        };
    }

    private void HandleWebMessageReceived(object? sender, CoreWebView2WebMessageReceivedEventArgs e)
    {
        try
        {
            using var message = JsonDocument.Parse(e.WebMessageAsJson);
            var root = message.RootElement;
            if (!root.TryGetProperty("type", out var typeElement))
            {
                return;
            }

            var messageType = typeElement.GetString();
            if (messageType == "permission-protector-branding")
            {
                ApplyRuntimeBranding(root);
                return;
            }

            if (messageType == "permission-protector-locale")
            {
                ApplyRuntimeLocale(root);
                return;
            }

            if (messageType != "permission-protector-window")
            {
                return;
            }

            if (!root.TryGetProperty("action", out var actionElement))
            {
                return;
            }

            switch (actionElement.GetString())
            {
                case "drag":
                    BeginWindowDrag();
                    break;
                case "resize-start":
                    if (
                        TryGetString(root, "direction", out var direction) &&
                        TryGetNumber(root, "screenX", out var startX) &&
                        TryGetNumber(root, "screenY", out var startY))
                    {
                        var scaleFactor = TryGetNumber(root, "scaleFactor", out var suppliedScale)
                            ? suppliedScale
                            : DeviceDpi / 96d;
                        BeginWebResize(direction, startX, startY, scaleFactor);
                    }
                    break;
                case "resize-move":
                    if (
                        TryGetNumber(root, "screenX", out var currentX) &&
                        TryGetNumber(root, "screenY", out var currentY))
                    {
                        ContinueWebResize(currentX, currentY);
                    }
                    break;
                case "resize-end":
                    activeWebResize = null;
                    break;
                case "minimize":
                    WindowState = FormWindowState.Minimized;
                    break;
                case "maximize":
                    WindowState = NativeWindowChrome.ToggleMaximized(WindowState);
                    break;
                case "close":
                    Close();
                    break;
            }
        }
        catch (JsonException)
        {
            // Ignore malformed messages from the embedded page.
        }
    }

    private void ApplyRuntimeBranding(JsonElement message)
    {
        if (!message.TryGetProperty("logoDataUrl", out var logoElement))
        {
            return;
        }

        var logoDataUrl = logoElement.GetString() ?? string.Empty;
        if (string.IsNullOrWhiteSpace(logoDataUrl))
        {
            ReplaceWindowIcon(DesktopBranding.CreateApplicationIcon(Application.ExecutablePath));
            return;
        }

        if (DesktopBranding.TryCreateRuntimeIcon(logoDataUrl, out var icon) && icon is not null)
        {
            ReplaceWindowIcon(icon);
        }
    }

    private void ApplyRuntimeLocale(JsonElement message)
    {
        if (TryGetString(message, "locale", out var locale))
        {
            StartupLocaleStore.TryPersist(runtime.DataDirectory, locale);
        }
    }

    private void ReplaceWindowIcon(Icon nextIcon)
    {
        var previousIcon = ownedWindowIcon;
        ownedWindowIcon = nextIcon;
        Icon = nextIcon;
        previousIcon?.Dispose();
    }

    private void BeginWindowDrag()
    {
        if (WindowState == FormWindowState.Maximized)
        {
            WindowState = FormWindowState.Normal;
        }

        ReleaseCapture();
        SendMessage(Handle, WM_NCLBUTTONDOWN, HTCAPTION, 0);
    }

    private void BeginWebResize(
        string direction,
        double screenX,
        double screenY,
        double scaleFactor)
    {
        if (
            WindowState != FormWindowState.Normal ||
            NativeWindowChrome.ResizeHitForDirection(direction) is null)
        {
            return;
        }

        activeWebResize = new WebResizeSession(
            Bounds,
            direction,
            screenX,
            screenY,
            Math.Clamp(scaleFactor, 0.5d, 4d));
    }

    private void ContinueWebResize(double screenX, double screenY)
    {
        var session = activeWebResize;
        if (session is null || WindowState != FormWindowState.Normal)
        {
            return;
        }

        var deltaX = (int)Math.Round((screenX - session.StartScreenX) * session.ScaleFactor);
        var deltaY = (int)Math.Round((screenY - session.StartScreenY) * session.ScaleFactor);
        Bounds = NativeWindowChrome.ResizeWindowBounds(
            session.InitialBounds,
            session.Direction,
            deltaX,
            deltaY,
            MinimumSize);
    }

    private static bool TryGetString(JsonElement message, string propertyName, out string value)
    {
        value = string.Empty;
        return message.TryGetProperty(propertyName, out var element) &&
            element.ValueKind == JsonValueKind.String &&
            !string.IsNullOrWhiteSpace(value = element.GetString() ?? string.Empty);
    }

    private static bool TryGetNumber(JsonElement message, string propertyName, out double value)
    {
        value = 0;
        return message.TryGetProperty(propertyName, out var element) &&
            element.ValueKind == JsonValueKind.Number &&
            element.TryGetDouble(out value);
    }

    private void UpdateMaximizedBounds()
    {
        MaximizedBounds = Screen.FromHandle(Handle).WorkingArea;
    }

    private void SetStatus(string message)
    {
        if (InvokeRequired)
        {
            BeginInvoke(() => SetStatus(message));
            return;
        }
        startupExperience.UpdateRuntimeStatus(message);
    }

    private sealed record WebResizeSession(
        Rectangle InitialBounds,
        string Direction,
        double StartScreenX,
        double StartScreenY,
        double ScaleFactor);
}
