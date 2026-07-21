namespace PermissionProtector.Desktop;

internal static class NativeWindowChrome
{
    internal const int WmNcCalcSize = 0x0083;
    internal const int WmNcHitTest = 0x0084;
    internal const int WsMaximizeBox = 0x00010000;
    internal const int WsMinimizeBox = 0x00020000;
    internal const int WsThickFrame = 0x00040000;
    internal const int WsSystemMenu = 0x00080000;
    internal const int WsCaption = 0x00C00000;
    internal const int HtClient = 1;
    internal const int HtLeft = 10;
    internal const int HtRight = 11;
    internal const int HtTop = 12;
    internal const int HtTopLeft = 13;
    internal const int HtTopRight = 14;
    internal const int HtBottom = 15;
    internal const int HtBottomLeft = 16;
    internal const int HtBottomRight = 17;

    private const int NativeResizeAndSystemStyles =
        WsMaximizeBox | WsMinimizeBox | WsThickFrame | WsSystemMenu;
    private const int ResizeGripAt96Dpi = 8;

    internal static int EnableBorderlessResizing(int style) =>
        (style & ~WsCaption) | NativeResizeAndSystemStyles;

    internal static int ResizeGripForDpi(int deviceDpi) =>
        Math.Max(ResizeGripAt96Dpi, (int)Math.Ceiling(ResizeGripAt96Dpi * Math.Max(deviceDpi, 96) / 96d));

    internal static int HitTestResizeBorder(Point cursor, Size clientSize, int deviceDpi)
    {
        var grip = ResizeGripForDpi(deviceDpi);
        var left = cursor.X >= 0 && cursor.X < grip;
        var right = cursor.X >= clientSize.Width - grip && cursor.X < clientSize.Width;
        var top = cursor.Y >= 0 && cursor.Y < grip;
        var bottom = cursor.Y >= clientSize.Height - grip && cursor.Y < clientSize.Height;

        if (left && top) return HtTopLeft;
        if (right && top) return HtTopRight;
        if (left && bottom) return HtBottomLeft;
        if (right && bottom) return HtBottomRight;
        if (left) return HtLeft;
        if (right) return HtRight;
        if (top) return HtTop;
        if (bottom) return HtBottom;
        return HtClient;
    }

    internal static int? ResizeHitForDirection(string? direction) =>
        direction switch
        {
            "left" => HtLeft,
            "right" => HtRight,
            "top" => HtTop,
            "bottom" => HtBottom,
            "top-left" => HtTopLeft,
            "top-right" => HtTopRight,
            "bottom-left" => HtBottomLeft,
            "bottom-right" => HtBottomRight,
            _ => null,
        };

    internal static Rectangle ResizeWindowBounds(
        Rectangle initialBounds,
        string direction,
        int deltaX,
        int deltaY,
        Size minimumSize)
    {
        if (ResizeHitForDirection(direction) is null)
        {
            return initialBounds;
        }

        var movesLeft = direction is "left" or "top-left" or "bottom-left";
        var movesRight = direction is "right" or "top-right" or "bottom-right";
        var movesTop = direction is "top" or "top-left" or "top-right";
        var movesBottom = direction is "bottom" or "bottom-left" or "bottom-right";

        var left = initialBounds.Left + (movesLeft ? deltaX : 0);
        var right = initialBounds.Right + (movesRight ? deltaX : 0);
        var top = initialBounds.Top + (movesTop ? deltaY : 0);
        var bottom = initialBounds.Bottom + (movesBottom ? deltaY : 0);

        var minimumWidth = Math.Max(1, minimumSize.Width);
        if (right - left < minimumWidth)
        {
            if (movesLeft)
            {
                left = right - minimumWidth;
            }
            else
            {
                right = left + minimumWidth;
            }
        }

        var minimumHeight = Math.Max(1, minimumSize.Height);
        if (bottom - top < minimumHeight)
        {
            if (movesTop)
            {
                top = bottom - minimumHeight;
            }
            else
            {
                bottom = top + minimumHeight;
            }
        }

        return Rectangle.FromLTRB(left, top, right, bottom);
    }

    internal static Point ScreenPointFromLParam(IntPtr lParam)
    {
        var value = lParam.ToInt64();
        return new Point(
            unchecked((short)(value & 0xffff)),
            unchecked((short)((value >> 16) & 0xffff)));
    }

    internal static bool TrySuppressNonClientFrame(ref Message message)
    {
        if (message.Msg != WmNcCalcSize || message.WParam == IntPtr.Zero)
        {
            return false;
        }

        message.Result = IntPtr.Zero;
        return true;
    }

    internal static FormWindowState ToggleMaximized(FormWindowState current) =>
        current == FormWindowState.Maximized
            ? FormWindowState.Normal
            : FormWindowState.Maximized;
}
