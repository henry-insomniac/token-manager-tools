(function bootstrapDesktopTransport(global) {
  const goBindings = global.go?.desktopapp?.Bindings;
  if (!goBindings) {
    return;
  }

  global.tokenManagerTransport = {
    async listProfiles() {
      const profiles = await goBindings.ListProfiles();
      return { profiles };
    },
    createProfile(name) {
      return goBindings.CreateProfile(name);
    },
    startLogin(name) {
      return goBindings.StartLogin(name);
    },
    completeManualLogin(profileName, input) {
      return goBindings.CompleteManualLogin(profileName, input);
    },
    probeProfile(name) {
      return goBindings.ProbeProfile(name);
    },
    activateProfile(name) {
      return goBindings.ActivateProfile(name);
    },
    removeProfile(name) {
      return goBindings.RemoveProfile(name);
    },
    refreshUsage() {
      return goBindings.RefreshUsage();
    },
    getAutoSwitchStatus() {
      return goBindings.GetAutoSwitchStatus();
    },
    setAutoSwitchEnabled(enabled) {
      return goBindings.SetAutoSwitchEnabled(enabled);
    },
    runAutoSwitchCheck() {
      return goBindings.RunAutoSwitchCheck();
    },
    getDesktopStatus() {
      return goBindings.GetDesktopStatus();
    },
    setDesktopAutoStart(enabled) {
      return goBindings.SetDesktopAutoStart(enabled);
    },
  };
})(window);
