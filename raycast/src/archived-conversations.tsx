import SessionBrowser from "./components/session-browser";

export default function Command() {
  return (
    <SessionBrowser title="Archived Conversations" statusFilter="archived" />
  );
}
