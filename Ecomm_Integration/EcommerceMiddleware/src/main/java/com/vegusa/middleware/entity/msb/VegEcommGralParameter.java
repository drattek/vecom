package com.vegusa.middleware.entity.msb;

import jakarta.persistence.*;

@Entity
@Table(name = "veg_ecomm_gral_parameters")
public class VegEcommGralParameter {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", nullable = false)
    private Long id;

    @Column(name = "parameter_name", nullable = false, length = 50)
    private String parameterName;

    @Lob
    @Column(name = "data_type")
    private String dataType;

    @Column(name = "str_value", length = 250)
    private String strValue;

    @Column(name = "int_value")
    private Integer intValue;

    @Column(name = "description", length = 250)
    private String description;

    @Column(name = "integration_company", nullable = false, length = 50)
    private String integrationCompany;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getParameterName() {
        return parameterName;
    }

    public void setParameterName(String parameterName) {
        this.parameterName = parameterName;
    }

    public String getDataType() {
        return dataType;
    }

    public void setDataType(String dataType) {
        this.dataType = dataType;
    }

    public String getStrValue() {
        return strValue;
    }

    public void setStrValue(String strValue) {
        this.strValue = strValue;
    }

    public Integer getIntValue() {
        return intValue;
    }

    public void setIntValue(Integer intValue) {
        this.intValue = intValue;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getIntegrationCompany() {
        return integrationCompany;
    }

    public void setIntegrationCompany(String integrationCompany) {
        this.integrationCompany = integrationCompany;
    }

}