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
    internal static readonly TimeSpan MinimumVisibleDuration = TimeSpan.FromMilliseconds(1800);

    private static readonly Color Surface = Color.FromArgb(15, 16, 20);
    private static readonly Color SurfaceLower = Color.FromArgb(11, 12, 15);
    private static readonly Color Primary = Color.FromArgb(129, 140, 248);
    private static readonly Color PrimaryMuted = Color.FromArgb(92, 101, 214);
    private static readonly Color Success = Color.FromArgb(52, 211, 153);
    private static readonly Color Danger = Color.FromArgb(248, 113, 113);
    private static readonly Color TextPrimary = Color.FromArgb(244, 246, 250);
    private static readonly Color TextSecondary = Color.FromArgb(177, 183, 197);
    private static readonly Color TextMuted = Color.FromArgb(119, 126, 142);
    private static readonly string[] StageLabels = ["运行时", "权限服务", "工作台", "桌面界面"];

    private readonly System.Windows.Forms.Timer animationTimer = new() { Interval = 32 };
    private readonly Stopwatch visibleTimer = new();
    private readonly Button retryButton = new();
    private readonly Button minimizeButton = new();
    private readonly Button closeButton = new();
    private StartupExperienceState state =
        StartupExperienceState.FromRuntimeMessage("Preparing desktop runtime...");
    private double displayedProgress;
    private double targetProgress = 0.08d;
    private double animationSeconds;

    internal event EventHandler? RetryRequested;
    internal event EventHandler? MinimizeRequested;
    internal event EventHandler? CloseRequested;
    internal event MouseEventHandler? DragRequested;

    internal StartupExperienceControl()
    {
        Dock = DockStyle.Fill;
        BackColor = Surface;
        DoubleBuffered = true;
        TabStop = false;
        AccessibleName = "OpenAD startup";
        AccessibleDescription = state.Headline;

        SetStyle(
            ControlStyles.AllPaintingInWmPaint |
            ControlStyles.OptimizedDoubleBuffer |
            ControlStyles.ResizeRedraw |
            ControlStyles.UserPaint,
            true);

        ConfigureWindowButton(minimizeButton, "−", "最小化窗口");
        ConfigureWindowButton(closeButton, "×", "关闭窗口");
        closeButton.FlatAppearance.MouseOverBackColor = Color.FromArgb(196, 43, 57);
        closeButton.FlatAppearance.MouseDownBackColor = Color.FromArgb(150, 32, 44);
        minimizeButton.Click += (_, _) => MinimizeRequested?.Invoke(this, EventArgs.Empty);
        closeButton.Click += (_, _) => CloseRequested?.Invoke(this, EventArgs.Empty);

        retryButton.Text = "重新启动";
        retryButton.AccessibleName = "重新启动 OpenAD";
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
            if (e.Button == MouseButtons.Left && e.Y <= 48)
            {
                DragRequested?.Invoke(this, e);
            }
        };

        BeginStartup();
    }

    internal void BeginStartup()
    {
        state = StartupExperienceState.FromRuntimeMessage("Preparing desktop runtime...");
        displayedProgress = 0d;
        targetProgress = state.TargetProgress;
        animationSeconds = 0d;
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
        state = StartupExperienceState.FromRuntimeMessage(runtimeMessage);
        targetProgress = state.IsFailure
            ? Math.Max(displayedProgress, state.TargetProgress)
            : Math.Max(targetProgress, state.TargetProgress);
        retryButton.Visible = state.IsFailure;
        AccessibleDescription = $"{state.Headline}. {state.Detail}";
        PerformLayout();
        Invalidate();
    }

    internal async Task CompleteAsync()
    {
        state = StartupExperienceState.Completed();
        targetProgress = 1d;
        AccessibleDescription = $"{state.Headline}. {state.Detail}";
        Invalidate();

        var remaining = MinimumVisibleDuration - visibleTimer.Elapsed;
        if (remaining > TimeSpan.Zero)
        {
            await Task.Delay(remaining);
        }
        await Task.Delay(180);
    }

    internal static StartupExperienceLayout CalculateLayout(Size size, bool showRetry)
    {
        var width = Math.Min(760f, Math.Max(560f, size.Width - 96f));
        var left = (size.Width - width) / 2f;
        var top = Math.Max(88f, size.Height * 0.15f);
        var brand = new RectangleF(left, top, width, 190f);
        var mark = new RectangleF(
            left + (width - 92f) / 2f,
            top,
            92f,
            92f);
        var status = new RectangleF(left + 40f, brand.Bottom + 14f, width - 80f, 88f);
        var retry = showRetry
            ? new RectangleF(left + (width - 128f) / 2f, status.Bottom + 20f, 128f, 38f)
            : new RectangleF(left + width / 2f, status.Bottom, 0f, 0f);
        var progress = new RectangleF(left, size.Height - 94f, width, 62f);

        return new(
            new RectangleF(left, top, width, progress.Bottom - top),
            brand,
            mark,
            status,
            retry,
            progress);
    }

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
        minimizeButton.SetBounds(Math.Max(0, Width - 92), 0, 46, 38);
        closeButton.SetBounds(Math.Max(0, Width - 46), 0, 46, 38);

        var layout = CalculateLayout(ClientSize, retryButton.Visible);
        retryButton.SetBounds(
            (int)layout.Retry.X,
            (int)layout.Retry.Y,
            (int)layout.Retry.Width,
            (int)layout.Retry.Height);

        minimizeButton.BringToFront();
        closeButton.BringToFront();
        retryButton.BringToFront();
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        var graphics = e.Graphics;
        graphics.SmoothingMode = SmoothingMode.AntiAlias;
        graphics.CompositingQuality = CompositingQuality.HighQuality;
        graphics.PixelOffsetMode = PixelOffsetMode.HighQuality;
        graphics.TextRenderingHint = TextRenderingHint.ClearTypeGridFit;

        var layout = CalculateLayout(ClientSize, retryButton.Visible);
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

        using var divider = new Pen(Color.FromArgb(28, 118, 125, 145), 1f);
        graphics.DrawLine(divider, 0f, 47f, Width, 47f);

        using var dotBrush = new SolidBrush(Color.FromArgb(18, 151, 158, 179));
        const int gap = 64;
        for (var y = 94; y < Height - 110; y += gap)
        {
            for (var x = 42; x < Width; x += gap)
            {
                graphics.FillEllipse(dotBrush, x, y, 1.5f, 1.5f);
            }
        }
    }

    private void DrawChrome(Graphics graphics)
    {
        using var productFont = new Font("Segoe UI", 9f, FontStyle.Bold);
        using var metaFont = new Font("Segoe UI", 8.5f, FontStyle.Regular);
        using var productBrush = new SolidBrush(TextSecondary);
        using var metaBrush = new SolidBrush(TextMuted);

        graphics.DrawString("OPENAD DESKTOP", productFont, productBrush, 24f, 16f);
        graphics.DrawString("OPEN SOURCE · LOCAL FIRST", metaFont, metaBrush, 146f, 17f);
        graphics.DrawString(
            "LOCAL RUNTIME",
            metaFont,
            metaBrush,
            24f,
            Height - 28f);
    }

    private void DrawBrand(Graphics graphics, StartupExperienceLayout layout)
    {
        var accent = state.IsFailure
            ? Danger
            : state.Phase == StartupExperiencePhase.Complete
                ? Success
                : Primary;

        DrawDirectoryMark(graphics, layout.Mark, accent);

        using var titleFont = new Font("Segoe UI Variable Display", 31f, FontStyle.Bold);
        using var subtitleFont = new Font("Segoe UI", 10f, FontStyle.Regular);
        using var titleBrush = new SolidBrush(TextPrimary);
        using var subtitleBrush = new SolidBrush(TextSecondary);
        using var centered = new StringFormat { Alignment = StringAlignment.Center };

        graphics.DrawString(
            PrimaryProductName,
            titleFont,
            titleBrush,
            new RectangleF(layout.Brand.X, layout.Mark.Bottom + 12f, layout.Brand.Width, 48f),
            centered);
        graphics.DrawString(
            "开放的 Active Directory 权限洞察与运维工作台",
            subtitleFont,
            subtitleBrush,
            new RectangleF(layout.Brand.X, layout.Mark.Bottom + 60f, layout.Brand.Width, 24f),
            centered);
    }

    private void DrawDirectoryMark(Graphics graphics, RectangleF bounds, Color accent)
    {
        var centerX = bounds.Left + bounds.Width / 2f;
        var topY = bounds.Top + 10f;
        var branchY = bounds.Top + 43f;
        var leafY = bounds.Bottom - 10f;
        var leftX = centerX - 28f;
        var rightX = centerX + 28f;
        var reveal = Math.Clamp(animationSeconds / 0.9d, 0d, 1d);

        using var basePen = new Pen(Color.FromArgb(62, 129, 138, 164), 2f)
        {
            StartCap = LineCap.Round,
            EndCap = LineCap.Round,
        };
        using var activePen = new Pen(Color.FromArgb(225, accent), 2.4f)
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
                centerX - 28f * (float)horizontal,
                branchY,
                centerX + 28f * (float)horizontal,
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

        using var rootFill = new SolidBrush(accent);
        using var leafFill = new SolidBrush(Color.FromArgb(215, TextPrimary));
        graphics.FillEllipse(rootFill, centerX - 5f, topY - 5f, 10f, 10f);
        foreach (var x in new[] { leftX, centerX, rightX })
        {
            graphics.FillEllipse(leafFill, x - 4f, leafY - 4f, 8f, 8f);
        }
    }

    private void DrawStatus(Graphics graphics, StartupExperienceLayout layout)
    {
        using var headlineFont = new Font("Segoe UI", 10.5f, FontStyle.Bold);
        using var detailFont = new Font("Segoe UI", 9f, FontStyle.Regular);
        using var headlineBrush = new SolidBrush(
            state.IsFailure
                ? Danger
                : state.Phase == StartupExperiencePhase.Complete
                    ? Success
                    : Primary);
        using var detailBrush = new SolidBrush(TextMuted);
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
            new RectangleF(layout.Status.X, layout.Status.Y, layout.Status.Width, 26f),
            centered);
        graphics.DrawString(
            state.Detail,
            detailFont,
            detailBrush,
            new RectangleF(
                layout.Status.X,
                layout.Status.Y + 32f,
                layout.Status.Width,
                layout.Status.Height - 32f),
            centered);
    }

    private void DrawProgress(Graphics graphics, StartupExperienceLayout layout)
    {
        var accent = state.IsFailure
            ? Danger
            : state.Phase == StartupExperiencePhase.Complete
                ? Success
                : PrimaryMuted;
        var lineY = layout.Progress.Y + 16f;

        using var basePen = new Pen(Color.FromArgb(58, 99, 106, 124), 2f);
        using var progressPen = new Pen(Color.FromArgb(220, accent), 2.5f)
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
        using var mutedBrush = new SolidBrush(TextMuted);
        using var activeBrush = new SolidBrush(TextSecondary);
        using var format = new StringFormat { Alignment = StringAlignment.Center };

        for (var index = 0; index < StageLabels.Length; index++)
        {
            var step = index / (StageLabels.Length - 1f);
            var x = layout.Progress.Left + layout.Progress.Width * step;
            var active = displayedProgress + 0.02d >= step;
            using var nodeFill = new SolidBrush(active ? accent : Color.FromArgb(58, 64, 77));
            graphics.FillEllipse(nodeFill, x - 4f, lineY - 4f, 8f, 8f);
            graphics.DrawString(
                StageLabels[index],
                active ? activeFont : labelFont,
                active ? activeBrush : mutedBrush,
                new RectangleF(x - 62f, lineY + 13f, 124f, 20f),
                format);
        }
    }
}
