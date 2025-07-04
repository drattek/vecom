package com.vegusa.middleware.integrations.mercadolibre.dto.user;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class UserMeliDTO {
    @JsonProperty("id")
    private String id;

    @JsonProperty("nickname")
    private String nickname;

    @JsonProperty("first_name")
    private String firstName;

    @JsonProperty("last_name")
    private String lastName;

    @JsonProperty("gender")
    private String gender;

    @JsonProperty("country_id")
    private String countryId;

    @JsonProperty("email")
    private String email;

    @JsonProperty("identification")
    private Identification identification;

    @JsonProperty("address")
    private Address address;

    @JsonProperty("phone")
    private Phone phone;

    @JsonProperty("alternative_phone")
    private Phone alternativePhone;

    @JsonProperty("user_type")
    private String userType;

    @JsonProperty("tags")
    private String[] tags;

    @JsonProperty("logo")
    private String logo;

    @JsonProperty("points")
    private Integer points;

    @JsonProperty("site_id")
    private String siteId;

    @JsonProperty("permalink")
    private String permalink;

    @JsonProperty("seller_experience")
    private String sellerExperience;

    @JsonProperty("company")
    private Company company;

    @JsonProperty("credit")
    private Credit credit;

    @JsonProperty("context")
    private Context context;

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private static class Identification{
        @JsonProperty("number")
        private String number;
        @JsonProperty("type")
        private String type;

        public Identification() {}

        public Identification(String number, String type) {
            this.number = number;
            this.type = type;
        }

        public String getNumber() {
            return number;
        }

        public void setNumber(String number) {
            this.number = number;
        }

        public String getType() {
            return type;
        }

        public void setType(String type) {
            this.type = type;
        }
    }

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private static class Address{
        @JsonProperty("address")
        private String address;
        @JsonProperty("city")
        private String city;
        @JsonProperty("state")
        private String state;
        @JsonProperty("zip_code")
        private String zipCode;

        public Address() {}

        public Address(String address, String city, String state, String zipCode) {
            this.address = address;
            this.city = city;
            this.state = state;
            this.zipCode = zipCode;
        }

        public String getAddress() {
            return address;
        }

        public void setAddress(String address) {
            this.address = address;
        }

        public String getCity() {
            return city;
        }

        public void setCity(String city) {
            this.city = city;
        }

        public String getState() {
            return state;
        }

        public void setState(String state) {
            this.state = state;
        }

        public String getZipCode() {
            return zipCode;
        }

        public void setZipCode(String zipCode) {
            this.zipCode = zipCode;
        }
    }

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private static class Phone{
        @JsonProperty("area_code")
        private String areaCode;
        @JsonProperty("extension")
        private String extension;
        @JsonProperty("number")
        private String number;

        public Phone() {}

        public Phone(String areaCode, String extension, String number) {
            this.areaCode = areaCode;
            this.extension = extension;
            this.number = number;
        }

        public String getAreaCode() {
            return areaCode;
        }

        public void setAreaCode(String areaCode) {
            this.areaCode = areaCode;
        }

        public String getExtension() {
            return extension;
        }

        public void setExtension(String extension) {
            this.extension = extension;
        }

        public String getNumber() {
            return number;
        }

        public void setNumber(String number) {
            this.number = number;
        }
    }

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private static class Company{
        @JsonProperty("brand_name")
        private String brandName;
        @JsonProperty("city_tax_id")
        private String cityTaxId;
        @JsonProperty("corporate_name")
        private String corporateName;
        @JsonProperty("identification")
        private String identification;
        @JsonProperty("cust_type_id")
        private String custTypeId;
        @JsonProperty("soft_descriptor")
        private String softDescriptor;

        public Company() {}

        public Company(String brandName, String cityTaxId, String corporateName, String identification, String custTypeId, String softDescriptor) {
            this.brandName = brandName;
            this.cityTaxId = cityTaxId;
            this.corporateName = corporateName;
            this.identification = identification;
            this.custTypeId = custTypeId;
            this.softDescriptor = softDescriptor;
        }

        public String getBrandName() {
            return brandName;
        }

        public void setBrandName(String brandName) {
            this.brandName = brandName;
        }

        public String getCityTaxId() {
            return cityTaxId;
        }

        public void setCityTaxId(String cityTaxId) {
            this.cityTaxId = cityTaxId;
        }

        public String getCorporateName() {
            return corporateName;
        }

        public void setCorporateName(String corporateName) {
            this.corporateName = corporateName;
        }

        public String getIdentification() {
            return identification;
        }

        public void setIdentification(String identification) {
            this.identification = identification;
        }

        public String getCustTypeId() {
            return custTypeId;
        }

        public void setCustTypeId(String custTypeId) {
            this.custTypeId = custTypeId;
        }

        public String getSoftDescriptor() {
            return softDescriptor;
        }

        public void setSoftDescriptor(String softDescriptor) {
            this.softDescriptor = softDescriptor;
        }
    }

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private static class Credit{
        @JsonProperty("consumed")
        private Integer consumed;
        @JsonProperty("credit_level_id")
        private String creditLevelId;
        @JsonProperty("rank")
        private String rank;

        public Credit() {}

        public Credit(Integer consumed, String creditLevelId, String rank) {
            this.consumed = consumed;
            this.creditLevelId = creditLevelId;
            this.rank = rank;
        }

        public Integer getConsumed() {
            return consumed;
        }

        public void setConsumed(Integer consumed) {
            this.consumed = consumed;
        }

        public String getCreditLevelId() {
            return creditLevelId;
        }

        public void setCreditLevelId(String creditLevelId) {
            this.creditLevelId = creditLevelId;
        }

        public String getRank() {
            return rank;
        }

        public void setRank(String rank) {
            this.rank = rank;
        }
    }

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private static class Context{
        @JsonProperty("device")
        private String device;
        @JsonProperty("flow")
        private String flow;
        @JsonProperty("ip_address")
        private String ipAddress;
        @JsonProperty("source")
        private String source;

        public Context() {}

        public Context(String device, String flow, String ipAddress, String source) {
            this.device = device;
            this.flow = flow;
            this.ipAddress = ipAddress;
            this.source = source;
        }

        public String getDevice() {
            return device;
        }

        public void setDevice(String device) {
            this.device = device;
        }

        public String getFlow() {
            return flow;
        }

        public void setFlow(String flow) {
            this.flow = flow;
        }

        public String getIpAddress() {
            return ipAddress;
        }

        public void setIpAddress(String ipAddress) {
            this.ipAddress = ipAddress;
        }

        public String getSource() {
            return source;
        }

        public void setSource(String source) {
            this.source = source;
        }
    }

    public UserMeliDTO() {}

    public UserMeliDTO(String id, String nickname, String firstName, String lastName, String gender, String countryId, String email, Identification identification, Address address, Phone phone, Phone alternativePhone, String userType, String[] tags, String logo, Integer points, String siteId, String permalink, String sellerExperience, Company company, Credit credit, Context context) {
        this.id = id;
        this.nickname = nickname;
        this.firstName = firstName;
        this.lastName = lastName;
        this.gender = gender;
        this.countryId = countryId;
        this.email = email;
        this.identification = identification;
        this.address = address;
        this.phone = phone;
        this.alternativePhone = alternativePhone;
        this.userType = userType;
        this.tags = tags;
        this.logo = logo;
        this.points = points;
        this.siteId = siteId;
        this.permalink = permalink;
        this.sellerExperience = sellerExperience;
        this.company = company;
        this.credit = credit;
        this.context = context;
    }

    public String getId() {
        return id;
    }

    public void setId(String id) {
        this.id = id;
    }

    public String getNickname() {
        return nickname;
    }

    public void setNickname(String nickname) {
        this.nickname = nickname;
    }

    public String getFirstName() {
        return firstName;
    }

    public void setFirstName(String firstName) {
        this.firstName = firstName;
    }

    public String getLastName() {
        return lastName;
    }

    public void setLastName(String lastName) {
        this.lastName = lastName;
    }

    public String getGender() {
        return gender;
    }

    public void setGender(String gender) {
        this.gender = gender;
    }

    public String getCountryId() {
        return countryId;
    }

    public void setCountryId(String countryId) {
        this.countryId = countryId;
    }

    public String getEmail() {
        return email;
    }

    public void setEmail(String email) {
        this.email = email;
    }

    public Identification getIdentification() {
        return identification;
    }

    public void setIdentification(Identification identification) {
        this.identification = identification;
    }

    public Address getAddress() {
        return address;
    }

    public void setAddress(Address address) {
        this.address = address;
    }

    public Phone getPhone() {
        return phone;
    }

    public void setPhone(Phone phone) {
        this.phone = phone;
    }

    public Phone getAlternativePhone() {
        return alternativePhone;
    }

    public void setAlternativePhone(Phone alternativePhone) {
        this.alternativePhone = alternativePhone;
    }

    public String getUserType() {
        return userType;
    }

    public void setUserType(String userType) {
        this.userType = userType;
    }

    public String[] getTags() {
        return tags;
    }

    public void setTags(String[] tags) {
        this.tags = tags;
    }

    public String getLogo() {
        return logo;
    }

    public void setLogo(String logo) {
        this.logo = logo;
    }

    public Integer getPoints() {
        return points;
    }

    public void setPoints(Integer points) {
        this.points = points;
    }

    public String getSiteId() {
        return siteId;
    }

    public void setSiteId(String siteId) {
        this.siteId = siteId;
    }

    public String getPermalink() {
        return permalink;
    }

    public void setPermalink(String permalink) {
        this.permalink = permalink;
    }

    public String getSellerExperience() {
        return sellerExperience;
    }

    public void setSellerExperience(String sellerExperience) {
        this.sellerExperience = sellerExperience;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

    public Credit getCredit() {
        return credit;
    }

    public void setCredit(Credit credit) {
        this.credit = credit;
    }

    public Context getContext() {
        return context;
    }

    public void setContext(Context context) {
        this.context = context;
    }
}
