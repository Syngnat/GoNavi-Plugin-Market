import { useEffect } from 'react';
import { Route, Routes, useLocation } from 'react-router-dom';
import { AuthProvider } from './auth/AuthContext';
import { Footer } from './components/Footer';
import { TopNav } from './components/TopNav';
import { AccountPage } from './pages/AccountPage';
import { AccountPluginPage } from './pages/AccountPluginPage';
import { CommunityPage } from './pages/CommunityPage';
import { DevelopersPage } from './pages/DevelopersPage';
import { HomePage } from './pages/HomePage';
import { LoginPage } from './pages/LoginPage';
import { PluginDetailPage } from './pages/PluginDetailPage';
import { PublishPage } from './pages/PublishPage';

function ScrollManager() {
  const { pathname, hash } = useLocation();
  useEffect(() => {
    if (hash) {
      const target = document.getElementById(hash.slice(1));
      if (target) {
        target.scrollIntoView();
        return;
      }
    }
    window.scrollTo(0, 0);
  }, [pathname, hash]);
  return null;
}

export default function App() {
  return (
    <AuthProvider>
      <div className="min-h-screen flex flex-col">
        <ScrollManager />
        <TopNav />
        <div className="flex-grow flex flex-col">
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/plugins/:id" element={<PluginDetailPage />} />
            <Route path="/developers" element={<DevelopersPage />} />
            <Route path="/community" element={<CommunityPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/publish" element={<PublishPage />} />
            <Route path="/account" element={<AccountPage view="overview" />} />
            <Route path="/account/plugins" element={<AccountPage view="plugins" />} />
            <Route path="/account/submissions" element={<AccountPage view="submissions" />} />
            <Route path="/account/plugins/:id" element={<AccountPluginPage />} />
            <Route path="*" element={<HomePage />} />
          </Routes>
        </div>
        <Footer />
      </div>
    </AuthProvider>
  );
}
