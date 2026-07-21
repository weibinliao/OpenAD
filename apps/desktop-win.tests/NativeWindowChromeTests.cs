using System.Drawing;
using System.Reflection;
using System.Windows.Forms;
using Xunit;

namespace PermissionProtector.Desktop.Tests;

public sealed class NativeWindowChromeTests
{
    [Fact]
    public void SuppressesCalculatedNonClientFrameForBorderlessWindow()
    {
        var message = Message.Create(
            IntPtr.Zero,
            NativeWindowChrome.WmNcCalcSize,
            new IntPtr(1),
            IntPtr.Zero);
        message.Result = new IntPtr(123);

        var handled = NativeWindowChrome.TrySuppressNonClientFrame(ref message);

        Assert.True(handled);
        Assert.Equal(IntPtr.Zero, message.Result);
    }

    [Theory]
    [InlineData(0x83, 0)]
    [InlineData(0x84, 1)]
    public void LeavesOtherWindowMessagesUntouched(int messageId, int wParam)
    {
        var message = Message.Create(
            IntPtr.Zero,
            messageId,
            new IntPtr(wParam),
            IntPtr.Zero);
        message.Result = new IntPtr(123);

        var handled = NativeWindowChrome.TrySuppressNonClientFrame(ref message);

        Assert.False(handled);
        Assert.Equal(new IntPtr(123), message.Result);
    }

    [Fact]
    public void MainFormOverridesCreateParamsForBorderlessResizing()
    {
        var property = typeof(MainForm).GetProperty(
            "CreateParams",
            BindingFlags.Instance | BindingFlags.NonPublic);

        Assert.NotNull(property);
        Assert.Equal(typeof(MainForm), property!.GetMethod!.DeclaringType);
    }

    [Fact]
    public void KeepsNativeResizeAndSystemStylesWhileRemovingTheCaption()
    {
        var style = NativeWindowChrome.EnableBorderlessResizing(0);

        Assert.Equal(0, style & NativeWindowChrome.WsCaption);
        Assert.Equal(NativeWindowChrome.WsThickFrame, style & NativeWindowChrome.WsThickFrame);
        Assert.Equal(NativeWindowChrome.WsSystemMenu, style & NativeWindowChrome.WsSystemMenu);
        Assert.Equal(NativeWindowChrome.WsMinimizeBox, style & NativeWindowChrome.WsMinimizeBox);
        Assert.Equal(NativeWindowChrome.WsMaximizeBox, style & NativeWindowChrome.WsMaximizeBox);
    }

    [Theory]
    [InlineData(2, 430, NativeWindowChrome.HtLeft)]
    [InlineData(1277, 430, NativeWindowChrome.HtRight)]
    [InlineData(640, 2, NativeWindowChrome.HtTop)]
    [InlineData(640, 857, NativeWindowChrome.HtBottom)]
    [InlineData(2, 2, NativeWindowChrome.HtTopLeft)]
    [InlineData(1277, 2, NativeWindowChrome.HtTopRight)]
    [InlineData(2, 857, NativeWindowChrome.HtBottomLeft)]
    [InlineData(1277, 857, NativeWindowChrome.HtBottomRight)]
    [InlineData(640, 430, NativeWindowChrome.HtClient)]
    public void ResolvesAllResizeEdgesAndCorners(int x, int y, int expected)
    {
        var hit = NativeWindowChrome.HitTestResizeBorder(
            new Point(x, y),
            new Size(1280, 860),
            deviceDpi: 96);

        Assert.Equal(expected, hit);
    }

    [Theory]
    [InlineData(96, 8)]
    [InlineData(120, 10)]
    [InlineData(144, 12)]
    [InlineData(192, 16)]
    public void ResizeGripScalesWithWindowDpi(int deviceDpi, int expected)
    {
        Assert.Equal(expected, NativeWindowChrome.ResizeGripForDpi(deviceDpi));
    }

    [Theory]
    [InlineData("left", NativeWindowChrome.HtLeft)]
    [InlineData("right", NativeWindowChrome.HtRight)]
    [InlineData("top", NativeWindowChrome.HtTop)]
    [InlineData("bottom", NativeWindowChrome.HtBottom)]
    [InlineData("top-left", NativeWindowChrome.HtTopLeft)]
    [InlineData("top-right", NativeWindowChrome.HtTopRight)]
    [InlineData("bottom-left", NativeWindowChrome.HtBottomLeft)]
    [InlineData("bottom-right", NativeWindowChrome.HtBottomRight)]
    public void MapsWebViewResizeDirectionsToNativeHitCodes(string direction, int expected)
    {
        Assert.Equal(expected, NativeWindowChrome.ResizeHitForDirection(direction));
    }

    [Theory]
    [InlineData("")]
    [InlineData("center")]
    [InlineData(null)]
    public void RejectsUnknownWebViewResizeDirections(string? direction)
    {
        Assert.Null(NativeWindowChrome.ResizeHitForDirection(direction));
    }

    [Theory]
    [InlineData("left", 200, 100, 900, 800)]
    [InlineData("right", 100, 100, 1100, 800)]
    [InlineData("top", 100, 150, 1000, 750)]
    [InlineData("bottom", 100, 100, 1000, 850)]
    [InlineData("top-left", 200, 150, 900, 750)]
    [InlineData("top-right", 100, 150, 1100, 750)]
    [InlineData("bottom-right", 100, 100, 1100, 850)]
    [InlineData("bottom-left", 200, 100, 900, 850)]
    public void AppliesWebViewResizeMovement(
        string direction,
        int expectedX,
        int expectedY,
        int expectedWidth,
        int expectedHeight)
    {
        var resized = NativeWindowChrome.ResizeWindowBounds(
            new Rectangle(100, 100, 1000, 800),
            direction,
            deltaX: 100,
            deltaY: 50,
            new Size(400, 300));

        Assert.Equal(
            new Rectangle(expectedX, expectedY, expectedWidth, expectedHeight),
            resized);
    }

    [Theory]
    [InlineData("left", 120, 100, 980, 800)]
    [InlineData("right", 100, 100, 980, 800)]
    [InlineData("top", 100, 220, 1000, 680)]
    [InlineData("bottom", 100, 100, 1000, 680)]
    public void ClampsWebViewResizeToMinimumWindowSize(
        string direction,
        int expectedX,
        int expectedY,
        int expectedWidth,
        int expectedHeight)
    {
        var resized = NativeWindowChrome.ResizeWindowBounds(
            new Rectangle(100, 100, 1000, 800),
            direction,
            deltaX: direction == "right" ? -100 : 100,
            deltaY: direction == "bottom" ? -200 : 200,
            new Size(980, 680));

        Assert.Equal(
            new Rectangle(expectedX, expectedY, expectedWidth, expectedHeight),
            resized);
    }

    [Theory]
    [InlineData(FormWindowState.Normal, FormWindowState.Maximized)]
    [InlineData(FormWindowState.Maximized, FormWindowState.Normal)]
    [InlineData(FormWindowState.Minimized, FormWindowState.Maximized)]
    public void TogglesMaximizedState(FormWindowState current, FormWindowState expected)
    {
        Assert.Equal(expected, NativeWindowChrome.ToggleMaximized(current));
    }
}
