import type { Metadata } from "next";
import { Header } from "@/components/layout/header";
import { ProfileForm } from "@/components/settings/profile-form";
import { PreferencesForm } from "@/components/settings/preferences-form";
import { CredentialsForm } from "@/components/settings/credentials-form";
import { NotificationsForm } from "@/components/settings/notifications-form";

export const metadata: Metadata = { title: "Settings" };

export default function SettingsPage() {
  return (
    <div className="flex flex-col h-full">
      <Header
        title="Settings"
        subtitle="Manage your profile, job preferences, and board credentials."
      />
      <div className="flex-1 px-6 pb-8">
        <div className="max-w-2xl space-y-6">
          <ProfileForm />
          <PreferencesForm />
          <CredentialsForm />
          <NotificationsForm />
        </div>
      </div>
    </div>
  );
}
