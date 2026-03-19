import SessionBrowser from "./components/session-browser";

export default function Command() {
  return <SessionBrowser title="Active Conversations" statusFilter="active" />;
}
