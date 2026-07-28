using Xunit;

namespace PermissionProtector.Desktop.Tests;

public sealed class SingleInstanceLockTests
{
    [Fact]
    public void UsesASessionLocalOpenADSpecificMutexName()
    {
        Assert.StartsWith(@"Local\OpenAD.Desktop.", SingleInstanceLock.MutexName, StringComparison.Ordinal);
        Assert.EndsWith(".SingleInstance.v1", SingleInstanceLock.MutexName, StringComparison.Ordinal);
        Assert.DoesNotContain("PermissionProtector", SingleInstanceLock.MutexName, StringComparison.Ordinal);
    }

    [Fact]
    public void AllowsOnlyOneOwnerAndCanBeReacquiredAfterRelease()
    {
        var mutexName = $@"Local\OpenAD.Desktop.Tests.{Guid.NewGuid():N}";
        var first = SingleInstanceLock.TryAcquire(mutexName);
        Assert.NotNull(first);

        SingleInstanceLock? contender = null;
        var contenderThread = new Thread(() => contender = SingleInstanceLock.TryAcquire(mutexName));
        contenderThread.Start();
        Assert.True(contenderThread.Join(TimeSpan.FromSeconds(5)));

        Assert.Null(contender);
        first!.Dispose();

        using var reacquired = SingleInstanceLock.TryAcquire(mutexName);
        Assert.NotNull(reacquired);
    }
}
