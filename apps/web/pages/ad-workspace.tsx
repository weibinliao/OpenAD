import { useEffect } from 'react';
import { useRouter } from 'next/router';

// Legacy route — the AD workspace was replaced by /identity in the redesign.
export default function ADWorkspaceRedirect() {
  const router = useRouter();
  useEffect(() => {
    router.replace('/identity');
  }, [router]);
  return null;
}
