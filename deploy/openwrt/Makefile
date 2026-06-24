include $(TOPDIR)/rules.mk

PKG_NAME:=outless
PKG_VERSION:=0.1.0
PKG_RELEASE:=1

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk

define Package/outless
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Outless proxy management panel
  DEPENDS:=+kmod-tun
  URL:=https://github.com/quonaro/outless
endef

define Package/outless/description
  Outless is a proxy management panel with embedded sing-box.
  This package expects a prebuilt binary for the target architecture.
endef

define Package/outless/conffiles
/etc/config/outless
/etc/outless/config.yaml
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	cp $(CURDIR)/outless $(PKG_BUILD_DIR)/outless
endef

define Build/Compile
	# Prebuilt binary — nothing to compile.
endef

define Package/outless/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/outless $(1)/usr/bin/outless

	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/outless.config $(1)/etc/config/outless

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/outless.init $(1)/etc/init.d/outless

	$(INSTALL_DIR) $(1)/etc/outless
endef

define Package/outless/postinst
#!/bin/sh
/etc/init.d/outless enable 2>/dev/null
exit 0
endef

define Package/outless/prerm
#!/bin/sh
/etc/init.d/outless stop 2>/dev/null
/etc/init.d/outless disable 2>/dev/null
exit 0
endef

$(eval $(call BuildPackage,outless))
