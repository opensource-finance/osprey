/* opensource.finance — full landing page composition. */
function Landing() {
  const { Header, Hero, Credibility, Features, TransactionDemo, Comparison, PlatformVision, FounderStory, Footer } = window;
  return (
    <div style={{ background: "var(--background)", color: "var(--foreground)", fontFamily: "var(--font-sans)", minHeight: "100vh" }}>
      <Header />
      <Hero />
      <Credibility />
      <Features />
      <TransactionDemo />
      <Comparison />
      <PlatformVision />
      <FounderStory />
      <Footer />
    </div>
  );
}
ReactDOM.createRoot(document.getElementById("root")).render(<Landing />);
