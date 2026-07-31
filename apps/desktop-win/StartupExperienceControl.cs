using System.Diagnostics;
using System.Drawing.Drawing2D;
using System.Drawing.Text;

namespace PermissionProtector.Desktop;

internal sealed record StartupExperienceLayout(
    RectangleF Content,
    RectangleF Brand,
    RectangleF Mark,
    RectangleF Status,
    RectangleF Retry,
    RectangleF Progress);

internal sealed class StartupExperienceControl : UserControl
{
    internal const string PrimaryProductName = "OpenAD";
    internal static readonly TimeSpan MinimumVisibleDuration = TimeSpan.FromMilliseconds(750);
    internal static readonly TimeSpan FadeOutDuration = TimeSpan.FromMilliseconds(240);
    internal static readonly string ApplicationVersionLabel = CreateApplicationVersionLabel();

    private static readonly Color Surface = Color.FromArgb(15, 16, 20);
    private static readonly Color SurfaceLower = Color.FromArgb(11, 12, 15);
    private static readonly Color Primary = Color.FromArgb(129, 140, 248);
    private static readonly Color PrimaryMuted = Color.FromArgb(92, 101, 214);
    private static readonly Color Success = Color.FromArgb(52, 211, 153);
    private static readonly Color Danger = Color.FromArgb(248, 113, 113);
    private static readonly Color TextPrimary = Color.FromArgb(244, 246, 250);
    private static readonly Color TextSecondary = Color.FromArgb(177, 183, 197);
    private static readonly Color TextMuted = Color.FromArgb(119, 126, 142);
    private static readonly string DisplayFontFamilyName = ResolveDisplayFontFamilyName();

    private readonly StartupExperienceLocale locale;
    private readonly StartupExperienceStrings strings;
    private readonly System.Windows.Forms.Timer animationTimer = new() { Interval = 32 };
    private readonly Stopwatch visibleTimer = new();
    private readonly Button retryButton = new();
    private readonly Button minimizeButton = new();
    private readonly Button closeButton = new();
    private StartupExperienceState state;
    private double displayedProgress;
    private double targetProgress = 0.08d;
    private double animationSeconds;
    private float transitionOpacity = 1f;

    internal event EventHandler? RetryRequested;
    internal event EventHandler? MinimizeRequested;
    internal event EventHandler? CloseRequested;
    internal event MouseEventHandler? DragRequested;

    internal StartupExperienceControl(StartupExperienceLocale locale)
    {
        this.locale = locale;
        strings = StartupExperienceStrings.For(locale);
        state = StartupExperienceState.FromRuntimeMessage("Preparing desktop runtime...", locale);
        Dock = DockStyle.Fill;
        BackColor = Surface;
        DoubleBuffered = true;
        TabStop = false;
        AccessibleName = strings.AccessibleName;
        AccessibleDescription = state.Headline;

        SetStyle(
            ControlStyles.AllPaintingInWmPaint |
            ControlStyles.OptimizedDoubleBuffer |
            ControlStyles.ResizeRedraw |
            ControlStyles.UserPaint,
            true);

        ConfigureWindowButton(minimizeButton, "−", strings.MinimizeAccessibleName);
        ConfigureWindowButton(closeButton, "×", strings.CloseAccessibleName);
        closeButton.FlatAppearance.MouseOverBackColor = Color.FromArgb(196, 43, 57);
        closeButton.FlatAppearance.MouseDownBackColor = Color.FromArgb(150, 32, 44);
        minimizeButton.Click += (_, _) => MinimizeRequested?.Invoke(this, EventArgs.Empty);
        closeButton.Click += (_, _) => CloseRequested?.Invoke(this, EventArgs.Empty);

        retryButton.Text = strings.RetryText;
        retryButton.AccessibleName = strings.RetryAccessibleName;
        retryButton.Size = new Size(128, 38);
        retryButton.FlatStyle = FlatStyle.Flat;
        retryButton.FlatAppearance.BorderSize = 1;
        retryButton.FlatAppearance.BorderColor = Color.FromArgb(69, 72, 84);
        retryButton.FlatAppearance.MouseOverBackColor = Color.FromArgb(31, 33, 40);
        retryButton.FlatAppearance.MouseDownBackColor = Color.FromArgb(25, 27, 33);
        retryButton.BackColor = Color.FromArgb(20, 21, 26);
        retryButton.ForeColor = TextPrimary;
        retryButton.Font = new Font("Segoe UI", 9.5f, FontStyle.Bold);
        retryButton.Cursor = Cursors.Hand;
        retryButton.Visible = false;
        retryButton.Click += (_, _) => RetryRequested?.Invoke(this, EventArgs.Empty);

        Controls.Add(retryButton);
        Controls.Add(minimizeButton);
        Controls.Add(closeButton);

        animationTimer.Tick += (_, _) =>
        {
            animationSeconds += animationTimer.Interval / 1000d;
            displayedProgress += (targetProgress - displayedProgress) * 0.12d;
            if (Math.Abs(targetProgress - displayedProgress) < 0.001d)
            {
                displayedProgress = targetProgress;
            }
            Invalidate();
        };

        MouseDown += (_, e) =>
        {
            if (e.Button == MouseButtons.Left && e.Y <= TitleBarHeightForDpi(DeviceDpi))
            {
                DragRequested?.Invoke(this, e);
            }
        };

        BeginStartup();
    }

    internal void BeginStartup()
    {
        state = StartupExperienceState.FromRuntimeMessage("Preparing desktop runtime...", locale);
        displayedProgress = 0d;
        targetProgress = state.TargetProgress;
        animationSeconds = 0d;
        transitionOpacity = 1f;
        ApplyTransitionToWindowButtons();
        retryButton.Visible = false;
        AccessibleDescription = state.Headline;
        visibleTimer.Restart();
        if (Visible)
        {
            animationTimer.Start();
        }
        PerformLayout();
        Invalidate();
    }

    internal void UpdateRuntimeStatus(string runtimeMessage)
    {
        state = StartupExperienceState.FromRuntimeMessage(runtimeMessage, locale);
        targetProgress = state.IsFailure
            ? Math.Max(displayedProgress, state.TargetProgress)
            : Math.Max(targetProgress, state.TargetProgress);
        retryButton.Visible = state.IsFailure;
        AccessibleDescription = $"{state.Headline}. {state.Detail}";
        PerformLayout();
        Invalidate();
    }

    internal async Task<bool> CompleteAsync()
    {
        if (state.IsFailure)
        {
            return false;
        }

        state = StartupExperienceState.Completed(locale);
        targetProgress = 1d;
        AccessibleDescription = $"{state.Headline}. {state.Detail}";
        Invalidate();

        var remaining = MinimumVisibleDuration - visibleTimer.Elapsed;
        if (remaining > TimeSpan.Zero)
        {
            await Task.Delay(remaining);
        }
        var fadeTimer = Stopwatch.StartNew();
        while (fadeTimer.Elapsed < FadeOutDuration)
        {
            var progress = (float)(fadeTimer.Elapsed.TotalMilliseconds / FadeOutDuration.TotalMilliseconds);
            transitionOpacity = 1f - Math.Clamp(progress, 0f, 1f);
            ApplyTransitionToWindowButtons();
            Invalidate();
            await Task.Delay(16);
        }

        transitionOpacity = 0f;
        ApplyTransitionToWindowButtons();
        Invalidate();
        return true;
    }

    internal static StartupExperienceLayout CalculateLayout(Size size, bool showRetry, int deviceDpi)
    {
        var scale = DpiScaleFor(deviceDpi);
        var width = Math.Min(760f * scale, Math.Max(560f * scale, size.Width - 96f * scale));
        var left = (size.Width - width) / 2f;
        var top = Math.Max(88f * scale, size.Height * 0.15f);
        var brand = new RectangleF(left, top, width, 190f * scale);
        var mark = new RectangleF(
            left + (width - 92f * scale) / 2f,
            top,
            92f * scale,
            92f * scale);
        var status = new RectangleF(
            left + 40f * scale,
            brand.Bottom + 14f * scale,
            width - 80f * scale,
            88f * scale);
        var retry = showRetry
            ? new RectangleF(
                left + (width - 128f * scale) / 2f,
                status.Bottom + 20f * scale,
                128f * scale,
                38f * scale)
            : new RectangleF(left + width / 2f, status.Bottom, 0f, 0f);
        var progress = new RectangleF(
            left,
            size.Height - 94f * scale,
            width,
            62f * scale);

        return new(
            new RectangleF(left, top, width, progress.Bottom - top),
            brand,
            mark,
            status,
            retry,
            progress);
    }

    internal static float TitleBarHeightForDpi(int deviceDpi) => 48f * DpiScaleFor(deviceDpi);

    protected override void OnVisibleChanged(EventArgs e)
    {
        base.OnVisibleChanged(e);
        if (Visible)
        {
            animationTimer.Start();
        }
        else
        {
            animationTimer.Stop();
        }
    }

    protected override void OnLayout(LayoutEventArgs e)
    {
        base.OnLayout(e);
        var buttonWidth = (int)Math.Round(ScaleToDpi(46f));
        var buttonHeight = (int)Math.Round(ScaleToDpi(38f));
        minimizeButton.SetBounds(Math.Max(0, Width - buttonWidth * 2), 0, buttonWidth, buttonHeight);
        closeButton.SetBounds(Math.Max(0, Width - buttonWidth), 0, buttonWidth, buttonHeight);

        var layout = CalculateLayout(ClientSize, retryButton.Visible, DeviceDpi);
        retryButton.SetBounds(
            (int)layout.Retry.X,
            (int)layout.Retry.Y,
            (int)layout.Retry.Width,
            (int)layout.Retry.Height);

        minimizeButton.BringToFront();
        closeButton.BringToFront();
        retryButton.BringToFront();
    }

    protected override void OnDpiChangedAfterParent(EventArgs e)
    {
        base.OnDpiChangedAfterParent(e);
        PerformLayout();
        Invalidate();
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        var graphics = e.Graphics;
        graphics.SmoothingMode = SmoothingMode.AntiAlias;
        graphics.CompositingQuality = CompositingQuality.HighQuality;
        graphics.PixelOffsetMode = PixelOffsetMode.HighQuality;
        graphics.TextRenderingHint = TextRenderingHint.ClearTypeGridFit;

        var layout = CalculateLayout(ClientSize, retryButton.Visible, DeviceDpi);
        DrawBackground(graphics);
        DrawChrome(graphics);
        DrawBrand(graphics, layout);
        DrawStatus(graphics, layout);
        DrawProgress(graphics, layout);
    }

    protected override void Dispose(bool disposing)
    {
        if (disposing)
        {
            animationTimer.Dispose();
        }
        base.Dispose(disposing);
    }

    private static void ConfigureWindowButton(Button button, string text, string accessibleName)
    {
        button.Text = text;
        button.AccessibleName = accessibleName;
        button.FlatStyle = FlatStyle.Flat;
        button.FlatAppearance.BorderSize = 0;
        button.FlatAppearance.MouseOverBackColor = Color.FromArgb(34, 35, 42);
        button.FlatAppearance.MouseDownBackColor = Color.FromArgb(42, 43, 51);
        button.BackColor = Surface;
        button.ForeColor = Color.FromArgb(205, 209, 220);
        button.Font = new Font("Segoe UI", 12f, FontStyle.Regular);
        button.TabStop = false;
        button.Cursor = Cursors.Hand;
    }

    private void DrawBackground(Graphics graphics)
    {
        using var background = new LinearGradientBrush(
            ClientRectangle,
            Surface,
            SurfaceLower,
            90f);
        graphics.FillRectangle(background, ClientRectangle);

        using var divider = new Pen(
            TransitionColor(Color.FromArgb(28, 118, 125, 145)),
            ScaleToDpi(1f));
        var dividerY = TitleBarHeightForDpi(DeviceDpi) - ScaleToDpi(1f);
        graphics.DrawLine(divider, 0f, dividerY, Width, dividerY);

        using var dotBrush = new SolidBrush(
            TransitionColor(Color.FromArgb(18, 151, 158, 179)));
        var gap = ScaleToDpi(64f);
        var dotSize = ScaleToDpi(1.5f);
        for (var y = ScaleToDpi(94f); y < Height - ScaleToDpi(110f); y += gap)
        {
            for (var x = ScaleToDpi(42f); x < Width; x += gap)
            {
                graphics.FillEllipse(dotBrush, x, y, dotSize, dotSize);
            }
        }
    }

    private void DrawChrome(Graphics graphics)
    {
        using var productFont = new Font("Segoe UI", 9f, FontStyle.Bold);
        using var metaFont = new Font("Segoe UI", 8.5f, FontStyle.Regular);
        using var productBrush = new SolidBrush(TransitionColor(TextSecondary));
        using var metaBrush = new SolidBrush(TransitionColor(TextMuted));

        var left = ScaleToDpi(24f);
        var productTop = ScaleToDpi(16f);
        const string desktopLabel = "OPENAD DESKTOP";
        graphics.DrawString(desktopLabel, productFont, productBrush, left, productTop);
        var productWidth = graphics.MeasureString(desktopLabel, productFont).Width;
        graphics.DrawString(
            strings.OpenSourceLocalFirst,
            metaFont,
            metaBrush,
            left + productWidth + ScaleToDpi(12f),
            ScaleToDpi(17f));
        graphics.DrawString(
            $"{strings.LocalRuntime} · {ApplicationVersionLabel}",
            metaFont,
            metaBrush,
            left,
            Height - ScaleToDpi(28f));
    }

    private void DrawBrand(Graphics graphics, StartupExperienceLayout layout)
    {
        var accent = state.IsFailure
            ? Danger
            : state.Phase == StartupExperiencePhase.Complete
                ? Success
                : Primary;

        DrawDirectoryMark(graphics, layout.Mark, accent);

        using var titleFont = new Font(DisplayFontFamilyName, 31f, FontStyle.Bold);
        using var subtitleFont = new Font("Segoe UI", 10f, FontStyle.Regular);
        using var titleBrush = new SolidBrush(TransitionColor(TextPrimary));
        using var subtitleBrush = new SolidBrush(TransitionColor(TextSecondary));
        using var centered = new StringFormat { Alignment = StringAlignment.Center };

        graphics.DrawString(
            PrimaryProductName,
            titleFont,
            titleBrush,
            new RectangleF(
                layout.Brand.X,
                layout.Mark.Bottom + ScaleToDpi(12f),
                layout.Brand.Width,
                ScaleToDpi(48f)),
            centered);
        graphics.DrawString(
            strings.Subtitle,
            subtitleFont,
            subtitleBrush,
            new RectangleF(
                layout.Brand.X,
                layout.Mark.Bottom + ScaleToDpi(60f),
                layout.Brand.Width,
                ScaleToDpi(24f)),
            centered);
    }

    private void DrawDirectoryMark(Graphics graphics, RectangleF bounds, Color accent)
    {
        var markScale = bounds.Width / 92f;
        var centerX = bounds.Left + bounds.Width / 2f;
        var topY = bounds.Top + 10f * markScale;
        var branchY = bounds.Top + 43f * markScale;
        var leafY = bounds.Bottom - 10f * markScale;
        var leftX = centerX - 28f * markScale;
        var rightX = centerX + 28f * markScale;
        var reveal = Math.Clamp(animationSeconds / 0.9d, 0d, 1d);

        using var basePen = new Pen(
            TransitionColor(Color.FromArgb(62, 129, 138, 164)),
            2f * markScale)
        {
            StartCap = LineCap.Round,
            EndCap = LineCap.Round,
        };
        using var activePen = new Pen(
            TransitionColor(Color.FromArgb(225, accent)),
            2.4f * markScale)
        {
            StartCap = LineCap.Round,
            EndCap = LineCap.Round,
        };

        graphics.DrawLine(basePen, centerX, topY, centerX, branchY);
        graphics.DrawLine(basePen, leftX, branchY, rightX, branchY);
        graphics.DrawLine(basePen, leftX, branchY, leftX, leafY);
        graphics.DrawLine(basePen, centerX, branchY, centerX, leafY);
        graphics.DrawLine(basePen, rightX, branchY, rightX, leafY);

        var trunkEnd = topY + (branchY - topY) * (float)Math.Min(1d, reveal * 1.8d);
        graphics.DrawLine(activePen, centerX, topY, centerX, trunkEnd);
        if (reveal > 0.45d)
        {
            var horizontal = Math.Clamp((reveal - 0.45d) / 0.35d, 0d, 1d);
            graphics.DrawLine(
                activePen,
                centerX - 28f * markScale * (float)horizontal,
                branchY,
                centerX + 28f * markScale * (float)horizontal,
                branchY);
        }
        if (reveal > 0.72d)
        {
            var vertical = Math.Clamp((reveal - 0.72d) / 0.28d, 0d, 1d);
            foreach (var x in new[] { leftX, centerX, rightX })
            {
                graphics.DrawLine(
                    activePen,
                    x,
                    branchY,
                    x,
                    branchY + (leafY - branchY) * (float)vertical);
            }
        }

        using var rootFill = new SolidBrush(TransitionColor(accent));
        using var leafFill = new SolidBrush(
            TransitionColor(Color.FromArgb(215, TextPrimary)));
        graphics.FillEllipse(
            rootFill,
            centerX - 5f * markScale,
            topY - 5f * markScale,
            10f * markScale,
            10f * markScale);
        foreach (var x in new[] { leftX, centerX, rightX })
        {
            graphics.FillEllipse(
                leafFill,
                x - 4f * markScale,
                leafY - 4f * markScale,
                8f * markScale,
                8f * markScale);
        }
    }

    private void DrawStatus(Graphics graphics, StartupExperienceLayout layout)
    {
        using var headlineFont = new Font("Segoe UI", 10.5f, FontStyle.Bold);
        using var detailFont = new Font("Segoe UI", 9f, FontStyle.Regular);
        using var headlineBrush = new SolidBrush(TransitionColor(
            state.IsFailure
                ? Danger
                : state.Phase == StartupExperiencePhase.Complete
                    ? Success
                    : Primary));
        using var detailBrush = new SolidBrush(TransitionColor(TextMuted));
        using var centered = new StringFormat
        {
            Alignment = StringAlignment.Center,
            LineAlignment = StringAlignment.Near,
            Trimming = StringTrimming.EllipsisCharacter,
        };

        graphics.DrawString(
            state.Headline,
            headlineFont,
            headlineBrush,
            new RectangleF(
                layout.Status.X,
                layout.Status.Y,
                layout.Status.Width,
                ScaleToDpi(26f)),
            centered);
        graphics.DrawString(
            state.Detail,
            detailFont,
            detailBrush,
            new RectangleF(
                layout.Status.X,
                layout.Status.Y + ScaleToDpi(32f),
                layout.Status.Width,
                layout.Status.Height - ScaleToDpi(32f)),
            centered);
    }

    private void DrawProgress(Graphics graphics, StartupExperienceLayout layout)
    {
        var accent = state.IsFailure
            ? Danger
            : state.Phase == StartupExperiencePhase.Complete
                ? Success
                : PrimaryMuted;
        var lineY = layout.Progress.Y + ScaleToDpi(16f);

        using var basePen = new Pen(
            TransitionColor(Color.FromArgb(58, 99, 106, 124)),
            ScaleToDpi(2f));
        using var progressPen = new Pen(
            TransitionColor(Color.FromArgb(220, accent)),
            ScaleToDpi(2.5f))
        {
            StartCap = LineCap.Round,
            EndCap = LineCap.Round,
        };
        graphics.DrawLine(basePen, layout.Progress.Left, lineY, layout.Progress.Right, lineY);
        graphics.DrawLine(
            progressPen,
            layout.Progress.Left,
            lineY,
            layout.Progress.Left + layout.Progress.Width * (float)Math.Clamp(displayedProgress, 0d, 1d),
            lineY);

        using var labelFont = new Font("Segoe UI", 8f, FontStyle.Regular);
        using var activeFont = new Font("Segoe UI", 8f, FontStyle.Bold);
        using var mutedBrush = new SolidBrush(TransitionColor(TextMuted));
        using var activeBrush = new SolidBrush(TransitionColor(TextSecondary));
        using var format = new StringFormat { Alignment = StringAlignment.Center };

        for (var index = 0; index < strings.StageLabels.Length; index++)
        {
            var step = index / (strings.StageLabels.Length - 1f);
            var x = layout.Progress.Left + layout.Progress.Width * step;
            var active = displayedProgress + 0.02d >= step;
            using var nodeFill = new SolidBrush(
                TransitionColor(active ? accent : Color.FromArgb(58, 64, 77)));
            var nodeRadius = ScaleToDpi(4f);
            graphics.FillEllipse(
                nodeFill,
                x - nodeRadius,
                lineY - nodeRadius,
                nodeRadius * 2f,
                nodeRadius * 2f);
            graphics.DrawString(
                strings.StageLabels[index],
                active ? activeFont : labelFont,
                active ? activeBrush : mutedBrush,
                new RectangleF(
                    x - ScaleToDpi(62f),
                    lineY + ScaleToDpi(13f),
                    ScaleToDpi(124f),
                    ScaleToDpi(20f)),
                format);
        }
    }

    private static float DpiScaleFor(int deviceDpi) => Math.Max(deviceDpi, 96) / 96f;

    private float ScaleToDpi(float value) => value * DpiScaleFor(DeviceDpi);

    private Color TransitionColor(Color color) => Color.FromArgb(
        (int)Math.Round(color.A * transitionOpacity),
        color.R,
        color.G,
        color.B);

    private void ApplyTransitionToWindowButtons()
    {
        var buttonForeground = Blend(Surface, Color.FromArgb(205, 209, 220), transitionOpacity);
        minimizeButton.ForeColor = buttonForeground;
        closeButton.ForeColor = buttonForeground;
    }

    private static Color Blend(Color background, Color foreground, float amount)
    {
        var ratio = Math.Clamp(amount, 0f, 1f);
        return Color.FromArgb(
            (int)Math.Round(background.R + (foreground.R - background.R) * ratio),
            (int)Math.Round(background.G + (foreground.G - background.G) * ratio),
            (int)Math.Round(background.B + (foreground.B - background.B) * ratio));
    }

    private static string ResolveDisplayFontFamilyName()
    {
        using var installedFonts = new InstalledFontCollection();
        foreach (var candidate in new[] { "Segoe UI Variable Display", "Segoe UI" })
        {
            if (installedFonts.Families.Any(
                family => family.Name.Equals(candidate, StringComparison.OrdinalIgnoreCase)))
            {
                return candidate;
            }
        }

        return FontFamily.GenericSansSerif.Name;
    }

    private static string CreateApplicationVersionLabel()
    {
        var version = typeof(StartupExperienceControl).Assembly.GetName().Version;
        return version is null
            ? "v0.0.0"
            : $"v{version.Major}.{version.Minor}.{Math.Max(0, version.Build)}";
    }
}
