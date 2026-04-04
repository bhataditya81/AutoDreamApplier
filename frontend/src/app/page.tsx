'use client';
import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { isAuthenticated } from '@/lib/auth';
import LandingPage from '@/components/marketing/landing-page';

export default function RootPage() {
  const router = useRouter();
  useEffect(() => {
    if (isAuthenticated()) {
      router.replace('/dashboard/overview');
    }
  }, [router]);
  return <LandingPage />;
}
